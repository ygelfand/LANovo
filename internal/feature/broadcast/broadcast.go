package broadcast

import (
	"context"
	"net/http"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/audio/tone"
	shared "github.com/ygelfand/libcountertop/pkg/media/broadcast"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/alarm"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/tones"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

var Get = sync.OnceValue(func() *shared.Broadcaster {
	return shared.New(shared.Dependencies{
		Settings: config.CallSection,
		Peers:    discovery.Get(),
		Sound:    device{},
		Tones:    tones.Get(),
		Turns:    voice.Get(),
		Busy:     busy,
		Messages: message.Get(),
		Client:   discovery.Client(),
	})
})

func init() {
	web.HandlePeer(
		"POST "+shared.Path,
		discovery.Home(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Get().Handler().ServeHTTP(w, r)
		})),
	)
}

func busy() bool {
	_, calling := call.Get().Now()
	return calling || alarm.Get().Ringing()
}

type device struct{}

func (device) Frames() (<-chan []int16, func()) { return mic.Get().Listen("broadcast") }

func (device) Play(pcm []int16) { speaker.Get().PlayVoice(pcm) }

func (device) Claim(name string, run func(context.Context) error) {
	speaker.Sound().
		Claim(name, func(ctx context.Context, _ *speaker.Speaker) error { return run(ctx) })
}

func (device) Tone(s config.Stream, level float64, snd tone.Sound) {
	speaker.Sound().Sound(s, level, snd)
}
