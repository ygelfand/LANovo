package call

import (
	"sync"

	sharedcall "github.com/ygelfand/libcountertop/pkg/media/call"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

type componentCall struct{ *sharedcall.Calls }

func (c *componentCall) Restore(v config.Config) { c.Calls.Restore(v.Call) }

var once sync.Once
var shared *sharedcall.Calls

func Get() *sharedcall.Calls {
	once.Do(func() {
		shared = sharedcall.New(sharedcall.Dependencies{
			Settings: config.CallSection,
			Peers:    discovery.Get(),
			Port:     web.Port,
			Shell:    shell.Get(),
			Privacy:  privacy.Get(),
			Media:    media.Get(),
			Sound:    device{},
			Camera:   sharedcall.NewCamera(livecam.Sessions()),
			Display:  display.Get(),
			Messages: message.Get(),
		})
	})
	return shared
}

func init() {
	component.Register(
		component.Network,
		func() *componentCall { return &componentCall{Get()} },
		component.Order(62),
	)
	web.Handle("POST /call/offer", Get().OfferHandler())
	web.Handle("POST /call/answer", Get().AnswerHandler())
	web.Handle("POST /call/end", Get().EndHandler())
}
