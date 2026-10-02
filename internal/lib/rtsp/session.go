package rtsp

import (
	"encoding/binary"
	"math/rand/v2"
	"net"
	"sync"
	"time"
)

const (
	queued = 256

	// RFC 3550 6.2 puts the minimum RTCP interval at 5 s.
	reportEvery = 5 * time.Second
)

type packet struct {
	b       []byte
	control bool
}

type session struct {
	id     string
	stream *Stream
	pz     packetizer
	base   uint32

	tcp     *conn
	channel byte

	udp  *net.UDPConn
	rtcp *net.UDPConn
	peer *net.UDPAddr

	mu       sync.Mutex
	started  bool
	playing  bool
	reported time.Time
	out      chan packet
	done     chan struct{}
	stop     sync.Once
}

func newSession(id string) *session {
	return &session{
		id:   id,
		pz:   packetizer{ssrc: rand.Uint32(), seq: uint16(rand.Uint32())},
		base: rand.Uint32(),
		out:  make(chan packet, queued),
		done: make(chan struct{}),
	}
}

func (s *session) play() {
	s.mu.Lock()
	if s.playing {
		s.mu.Unlock()
		return
	}
	s.playing = true
	s.mu.Unlock()
	go s.send()
	s.stream.add(s)
}

func (s *session) close() {
	s.stop.Do(func() {
		if s.stream != nil {
			s.stream.remove(s)
		}
		close(s.done)
		if s.udp != nil {
			s.udp.Close()
		}
		if s.rtcp != nil {
			s.rtcp.Close()
		}
	})
}

func (s *session) deliver(nalus [][]byte, key bool, sps, pps []byte, pts time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		if !key {
			return
		}
		s.started = true
		if nalType(nalus[0]) != nalSPS && len(sps) > 0 && len(pps) > 0 {
			nalus = append([][]byte{sps, pps}, nalus...)
		}
	}
	ts := s.base + uint32(int64(pts)*9/100000)
	for _, p := range s.pz.packets(nalus, ts) {
		if !s.queue(packet{b: p}) {
			return
		}
	}
	if now := time.Now(); now.Sub(s.reported) >= reportEvery {
		s.reported = now
		s.queue(packet{b: s.pz.report(now, ts, "lanovo-"+s.id), control: true})
	}
}

func (s *session) queue(p packet) bool {
	select {
	case s.out <- p:
		return true
	default:
		s.started = false
		return false
	}
}

func (s *session) send() {
	for {
		select {
		case <-s.done:
			return
		case p := <-s.out:
			var err error
			switch {
			case s.tcp != nil && p.control:
				err = s.tcp.interleaved(s.channel+1, p.b)
			case s.tcp != nil:
				err = s.tcp.interleaved(s.channel, p.b)
			case p.control:
				s.rtcp.WriteToUDP(p.b, &net.UDPAddr{IP: s.peer.IP, Port: s.peer.Port + 1, Zone: s.peer.Zone})
			default:
				_, err = s.udp.WriteToUDP(p.b, s.peer)
			}
			if err != nil {
				s.close()
				return
			}
		}
	}
}

func (s *session) rtpInfo() (uint16, uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pz.seq, s.base
}

func frame(channel byte, p []byte) []byte {
	b := make([]byte, 4, 4+len(p))
	b[0] = '$'
	b[1] = channel
	binary.BigEndian.PutUint16(b[2:], uint16(len(p)))
	return append(b, p...)
}
