package rtsp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// Stream is one H.264 source. Everything written to it goes to every session playing it.
type Stream struct {
	// OnDemand is called when a client asks for the stream, so its source can start.
	OnDemand func()

	mu       sync.Mutex
	sps, pps []byte
	subs     map[*session]bool
	ready    chan struct{}
}

func NewStream() *Stream { return &Stream{subs: map[*session]bool{}, ready: make(chan struct{})} }

// Write hands over one access unit in Annex-B form, stamped with its presentation time.
func (st *Stream) Write(au []byte, pts time.Duration) {
	var body [][]byte
	key := false
	st.mu.Lock()
	for _, n := range NALUs(au) {
		switch nalType(n) {
		case nalAUD:
			continue
		case nalSPS:
			if !bytes.Equal(st.sps, n) {
				st.sps = append([]byte(nil), n...)
			}
		case nalPPS:
			if !bytes.Equal(st.pps, n) {
				st.pps = append([]byte(nil), n...)
			}
		case nalIDR:
			key = true
		}
		body = append(body, n)
	}
	subs := make([]*session, 0, len(st.subs))
	for s := range st.subs {
		subs = append(subs, s)
	}
	sps, pps := st.sps, st.pps
	if len(sps) >= 4 && len(pps) > 0 {
		select {
		case <-st.ready:
		default:
			close(st.ready)
		}
	}
	st.mu.Unlock()

	if len(body) == 0 {
		return
	}
	for _, s := range subs {
		s.deliver(body, key, sps, pps, pts)
	}
}

func (st *Stream) demand() {
	if st.OnDemand != nil {
		st.OnDemand()
	}
}

func (st *Stream) add(s *session) {
	st.mu.Lock()
	st.subs[s] = true
	st.mu.Unlock()
}

func (st *Stream) remove(s *session) {
	st.mu.Lock()
	delete(st.subs, s)
	st.mu.Unlock()
}

// Viewers is how many sessions are playing.
func (st *Stream) Viewers() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.subs)
}

func (st *Stream) sdp(id uint32, host string, wait time.Duration) (string, bool) {
	select {
	case <-st.ready:
	case <-time.After(wait):
		return "", false
	}
	st.mu.Lock()
	sps, pps := st.sps, st.pps
	st.mu.Unlock()
	if len(sps) < 4 || len(pps) == 0 {
		return "", false
	}
	enc := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("v=0\r\n"+
		"o=- %d 1 IN IP4 %s\r\n"+
		"s=LANovo\r\n"+
		"c=IN IP4 0.0.0.0\r\n"+
		"t=0 0\r\n"+
		"m=video 0 RTP/AVP %d\r\n"+
		"a=rtpmap:%d H264/90000\r\n"+
		"a=fmtp:%d packetization-mode=1;profile-level-id=%02X%02X%02X;sprop-parameter-sets=%s,%s\r\n"+
		"a=control:trackID=0\r\n",
		id, host, payloadType, payloadType, payloadType, sps[1], sps[2], sps[3], enc(sps), enc(pps)), true
}
