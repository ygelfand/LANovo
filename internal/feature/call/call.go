package call

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/callvideo"
	"github.com/ygelfand/libcountertop/pkg/display/callview"
	sharedcall "github.com/ygelfand/libcountertop/pkg/media/call"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

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
			Privacy:  privacy.Get(),
			Media:    media.Get(),
			Sound:    device{},
			Stage:    callview.NewStage(shell.Get()),
			Video: callvideo.New(callvideo.Dependencies{
				Display: display.Get(),
				Camera:  callvideo.NewCamera(livecam.Get().Sessions()),
			}),
			Messages: message.Get(),
		})
		component.Settings.Add(shared.Controls())
	})
	return shared
}

func init() {
	component.Register(
		sharedcomponent.Network,
		func() *componentCall { return &componentCall{Get()} },
		sharedcomponent.Order(62),
	)
	web.Handle("POST /call/offer", Get().OfferHandler())
	web.Handle("POST /call/answer", Get().AnswerHandler())
	web.Handle("POST /call/end", Get().EndHandler())
}
