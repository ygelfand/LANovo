package rtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/opus"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
)

const (
	FrameSamples = 320
	frame        = 20 * time.Millisecond
	PlayRate     = 48000
	PlayChannels = 2
	maxConceal   = 5
	opusType     = 111
	h264Type     = 102
	videoClock   = 90000
	videoLate    = 256
	videoFrame   = 33 * time.Millisecond
)

type Audio interface {
	Frames() (<-chan []int16, func())
	Play(pcm []int16)
}

type Frame struct {
	Data []byte
	PTS  time.Duration
	Key  bool
}

type Camera interface {
	Frames() (<-chan Frame, func())
}

type Screen interface {
	Show(Frame)
}

type Message struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data,omitempty"`
}

type Media struct {
	Audio    Audio
	Camera   Camera
	Screen   Screen
	Opened   func()
	Received func(Message)
}

var h264 = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: videoClock, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"}

type Stats struct {
	AudioSent, AudioRecv, AudioLost uint64
	VideoSent, VideoRecv, VideoKeys uint64
	VideoWaiting                    uint64
	EncodeTotal                     time.Duration
}

func (s Stats) EncodeAverage() time.Duration {
	if s.AudioSent == 0 {
		return 0
	}
	return s.EncodeTotal / time.Duration(s.AudioSent)
}

type counters struct {
	audioSent, audioRecv, audioLost atomic.Uint64
	videoSent, videoRecv, videoKeys atomic.Uint64
	videoWaiting                    atomic.Uint64
	encode                          atomic.Int64
}

type Link struct {
	count counters

	pc    *webrtc.PeerConnection
	track *webrtc.TrackLocalStaticSample
	video *webrtc.TrackLocalStaticSample
	talk  *webrtc.DataChannel
	media Media
	ended func(webrtc.PeerConnectionState)

	muted   atomic.Bool
	blind   atomic.Bool
	sending sync.Once
	stop    context.CancelFunc
	ctx     context.Context
	closed  sync.Once
}

func api() (*webrtc.API, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"},
		PayloadType:        opusType,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: h264, PayloadType: h264Type}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	reg := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, reg); err != nil {
		return nil, err
	}
	se := webrtc.SettingEngine{}
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	return webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(reg), webrtc.WithSettingEngine(se)), nil
}

func New(m Media, ended func(webrtc.PeerConnectionState)) (*Link, error) {
	built, err := api()
	if err != nil {
		return nil, err
	}
	pc, err := built.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}
	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "lanovo")
	if err != nil {
		pc.Close()
		return nil, err
	}
	sender, err := pc.AddTrack(track)
	if err != nil {
		pc.Close()
		return nil, err
	}
	ctx, stop := context.WithCancel(context.Background())
	l := &Link{pc: pc, track: track, media: m, ended: ended, ctx: ctx, stop: stop}
	go l.drain(sender)
	switch {
	case m.Camera != nil:
		l.video, err = webrtc.NewTrackLocalStaticSample(h264, "video", "lanovo")
		if err == nil {
			var vs *webrtc.RTPSender
			if vs, err = pc.AddTrack(l.video); err == nil {
				go l.drain(vs)
			}
		}
	case m.Screen != nil:
		_, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	}
	if err == nil {
		negotiated, id := true, uint16(0)
		l.talk, err = pc.CreateDataChannel("status", &webrtc.DataChannelInit{Negotiated: &negotiated, ID: &id})
	}
	if err != nil {
		pc.Close()
		stop()
		return nil, err
	}
	l.talk.OnOpen(func() {
		if m.Opened != nil {
			m.Opened()
		}
	})
	l.talk.OnMessage(l.heard)
	pc.OnTrack(l.receive)
	pc.OnConnectionStateChange(l.state)
	return l, nil
}

func (l *Link) Offer(ctx context.Context) (string, error) {
	offer, err := l.pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}
	return l.settle(ctx, offer)
}

func (l *Link) Accept(ctx context.Context, offer string) (string, error) {
	if err := l.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		return "", err
	}
	answer, err := l.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	return l.settle(ctx, answer)
}

func (l *Link) Answered(answer string) error {
	return l.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer})
}

func (l *Link) settle(ctx context.Context, desc webrtc.SessionDescription) (string, error) {
	gathered := webrtc.GatheringCompletePromise(l.pc)
	if err := l.pc.SetLocalDescription(desc); err != nil {
		return "", err
	}
	select {
	case <-gathered:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return l.pc.LocalDescription().SDP, nil
}

func (l *Link) Mute(on bool) { l.muted.Store(on) }

func (l *Link) Muted() bool { return l.muted.Load() }

func (l *Link) CameraOff(on bool) { l.blind.Store(on) }

func (l *Link) Sending() bool { return l.video != nil && !l.blind.Load() }

var ErrNotOpen = errors.New("rtc: data channel not open")

func (l *Link) Send(kind string, v any) error {
	if l.talk.ReadyState() != webrtc.DataChannelStateOpen {
		return ErrNotOpen
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b, err := json.Marshal(Message{Kind: kind, Data: data})
	if err != nil {
		return err
	}
	return l.talk.SendText(string(b))
}

func (l *Link) heard(msg webrtc.DataChannelMessage) {
	var m Message
	if err := json.Unmarshal(msg.Data, &m); err != nil || m.Kind == "" || l.media.Received == nil {
		return
	}
	l.media.Received(m)
}

func (l *Link) Close() error {
	var err error
	l.closed.Do(func() {
		l.stop()
		err = l.pc.Close()
	})
	return err
}

func (l *Link) state(s webrtc.PeerConnectionState) {
	slog.Debug("call link", "state", s)
	switch s {
	case webrtc.PeerConnectionStateConnected:
		l.sending.Do(func() {
			go l.send()
			if l.video != nil {
				go l.film()
			}
		})
	case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
		if l.ended != nil {
			l.ended(s)
		}
	}
}

func (l *Link) drain(sender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buf); err != nil {
			return
		}
	}
}

func (l *Link) send() {
	enc, err := opus.NewEncoder(opus.WithChannels(1), opus.WithApplication(opus.ApplicationVoIP))
	if err != nil {
		slog.Error("call encoder", "err", err)
		return
	}
	frames, done := l.media.Audio.Frames()
	defer done()
	silence := make([]int16, FrameSamples)
	out := make([]byte, 1275)
	for {
		select {
		case <-l.ctx.Done():
			return
		case pcm, ok := <-frames:
			if !ok {
				return
			}
			if l.muted.Load() || len(pcm) != FrameSamples {
				pcm = silence
			}
			began := time.Now()
			n, err := enc.EncodeSILK(pcm, opus.BandwidthWideband, out)
			l.count.encode.Add(int64(time.Since(began)))
			l.count.audioSent.Add(1)
			if err != nil {
				slog.Warn("call encode", "err", err)
				continue
			}
			if err := l.track.WriteSample(media.Sample{Data: append([]byte(nil), out[:n]...), Duration: frame}); err != nil && !errors.Is(err, context.Canceled) {
				slog.Debug("call send", "err", err)
			}
		}
	}
}

func (l *Link) receive(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
	switch {
	case t.Kind() == webrtc.RTPCodecTypeVideo && l.media.Screen != nil:
		l.watch(t)
		return
	case t.Kind() != webrtc.RTPCodecTypeAudio:
		return
	}
	dec, err := opus.NewDecoderWithOutput(PlayRate, PlayChannels)
	if err != nil {
		slog.Error("call decoder", "err", err)
		return
	}
	out := make([]int16, 5760*PlayChannels)
	conceal := out[:PlayRate/50*PlayChannels]
	var last uint16
	started := false
	for {
		p, _, err := t.ReadRTP()
		if err != nil {
			return
		}
		if started {
			step := p.SequenceNumber - last
			if int16(step) <= 0 {
				continue
			}
			if gap := int(step) - 1; gap > 0 {
				l.count.audioLost.Add(uint64(gap))
			}
			if gap := int(step) - 1; gap > 0 && gap <= maxConceal {
				for range gap {
					if dec.DecodePLC(conceal) == nil {
						l.media.Audio.Play(append([]int16(nil), conceal...))
					}
				}
			}
		}
		started, last = true, p.SequenceNumber
		l.count.audioRecv.Add(1)
		n, err := dec.DecodeToInt16(p.Payload, out)
		if err != nil {
			slog.Debug("call decode", "err", err)
			continue
		}
		l.media.Audio.Play(append([]int16(nil), out[:n*PlayChannels]...))
	}
}

func (l *Link) film() {
	frames, done := l.media.Camera.Frames()
	defer done()
	var last time.Duration
	waiting := true
	for {
		select {
		case <-l.ctx.Done():
			return
		case f, ok := <-frames:
			if !ok {
				return
			}
			if l.blind.Load() {
				waiting = true
				continue
			}
			if waiting && !f.Key {
				continue
			}
			waiting = false
			step := f.PTS - last
			if last == 0 || step <= 0 || step > time.Second {
				step = videoFrame
			}
			last = f.PTS
			l.count.videoSent.Add(1)
			if err := l.video.WriteSample(media.Sample{Data: f.Data, Duration: step}); err != nil {
				slog.Debug("call video send", "err", err)
			}
		}
	}
}

func (l *Link) watch(t *webrtc.TrackRemote) {
	sb := samplebuilder.New(videoLate, &codecs.H264Packet{}, videoClock)
	var first uint32
	started, broken := false, true
	for {
		p, _, err := t.ReadRTP()
		if err != nil {
			return
		}
		sb.Push(p)
		for s := sb.Pop(); s != nil; s = sb.Pop() {
			key := Keyframe(s.Data)
			l.count.videoRecv.Add(1)
			if key {
				l.count.videoKeys.Add(1)
			}
			if s.PrevDroppedPackets > 0 {
				broken = true
			}
			if broken && !key {
				l.count.videoWaiting.Add(1)
				l.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(t.SSRC())}})
				continue
			}
			broken = false
			if !started {
				first, started = s.PacketTimestamp, true
			}
			pts := time.Duration(s.PacketTimestamp-first) * time.Second / videoClock
			l.media.Screen.Show(Frame{Data: s.Data, PTS: pts, Key: key})
		}
	}
}

func Keyframe(annexB []byte) bool {
	for i := 0; i+3 < len(annexB); i++ {
		if annexB[i] == 0 && annexB[i+1] == 0 && annexB[i+2] == 1 {
			switch annexB[i+3] & 0x1f {
			case 5, 7:
				return true
			}
		}
	}
	return false
}

func (l *Link) Stats() Stats {
	c := &l.count
	return Stats{
		AudioSent: c.audioSent.Load(), AudioRecv: c.audioRecv.Load(), AudioLost: c.audioLost.Load(),
		VideoSent: c.videoSent.Load(), VideoRecv: c.videoRecv.Load(), VideoKeys: c.videoKeys.Load(),
		VideoWaiting: c.videoWaiting.Load(), EncodeTotal: time.Duration(c.encode.Load()),
	}
}

func (l *Link) String() string { return fmt.Sprintf("link(%s)", l.pc.ConnectionState()) }
