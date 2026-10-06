package call

import (
	"log/slog"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

type entities struct {
	incoming *esphome.Switch
	auto     *esphome.Switch
	wake     *esphome.Switch
	stream   *esphome.Select
	video    *esphome.Switch
	state    *esphome.TextSensor
	peer     *esphome.TextSensor
}

func (c *Calls) build() {
	c.ha = entities{
		incoming: &esphome.Switch{Base: esphome.Base{ObjectID: "call_incoming", Name: "Incoming calls", Icon: "mdi:phone-incoming", Category: esphome.CategoryConfig}},
		auto:     &esphome.Switch{Base: esphome.Base{ObjectID: "call_auto_answer", Name: "Auto-answer calls", Icon: "mdi:phone-in-talk", Category: esphome.CategoryConfig}},
		wake:     &esphome.Switch{Base: esphome.Base{ObjectID: "call_pause_wake", Name: "Pause wake word during calls", Icon: "mdi:microphone-off", Category: esphome.CategoryConfig}},
		video:    &esphome.Switch{Base: esphome.Base{ObjectID: "call_auto_video", Name: "Auto-answer with camera on", Icon: "mdi:video", Category: esphome.CategoryConfig}},
		stream:   &esphome.Select{Base: esphome.Base{ObjectID: "call_stream", Name: "Call video stream", Icon: "mdi:video-box", Category: esphome.CategoryConfig}, Options: config.Labels(config.CallStreams())},
		state:    &esphome.TextSensor{Base: esphome.Base{ObjectID: "call_state", Name: "Call", Icon: "mdi:phone"}},
		peer:     &esphome.TextSensor{Base: esphome.Base{ObjectID: "call_peer", Name: "Call with", Icon: "mdi:account-voice"}},
	}
	c.ha.incoming.OnCommand = SetIncoming
	c.ha.auto.OnCommand = SetAutoAnswer
	c.ha.wake.OnCommand = SetPauseWake
	c.ha.video.OnCommand = SetAutoVideo
	c.ha.stream.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.CallStreams(), l); ok {
			SetStream(v)
		}
	}
}

func (c *Calls) Entities() []esphome.Entity {
	return []esphome.Entity{c.ha.incoming, c.ha.auto, c.ha.wake, c.ha.video, c.ha.stream, c.ha.state, c.ha.peer}
}

func (c *Calls) Restore(cfg config.Config) {
	c.ha.incoming.Set(cfg.Call.Incoming)
	c.ha.auto.Set(cfg.Call.AutoAnswer)
	c.ha.wake.Set(cfg.Call.PauseWake)
	c.ha.video.Set(cfg.Call.AutoVideo)
	c.ha.stream.Set(cfg.Call.Stream.Label())
	c.publish(nil)
}

func (c *Calls) publish(s *session) {
	if s == nil {
		c.ha.state.Set(Idle.String())
		c.ha.peer.Set("")
		return
	}
	c.mu.Lock()
	state, peer := s.State, s.Peer.Name
	c.mu.Unlock()
	c.ha.state.Set(state.String())
	c.ha.peer.Set(peer)
}

func save(what string, err error) {
	if err != nil {
		slog.Error("saving a call setting failed", "setting", what, "err", err)
	}
	cfg := config.Get().Call
	c := Get()
	c.ha.incoming.Set(cfg.Incoming)
	c.ha.auto.Set(cfg.AutoAnswer)
	c.ha.wake.Set(cfg.PauseWake)
	c.ha.video.Set(cfg.AutoVideo)
	c.ha.stream.Set(cfg.Stream.Label())
	shell.Get().Redraw()
}

func SetIncoming(on bool) { save("incoming", config.Set().Call().Incoming(on)) }

func SetAutoAnswer(on bool) { save("auto_answer", config.Set().Call().AutoAnswer(on)) }

func SetPauseWake(on bool) { save("pause_wake", config.Set().Call().PauseWake(on)) }

func SetAutoVideo(on bool) { save("auto_video", config.Set().Call().AutoVideo(on)) }

func SetStream(v config.CallStream) { save("stream", config.Set().Call().Stream(v)) }
