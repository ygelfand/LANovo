package call

import (
	"context"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/rtc"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	sharedcall "github.com/ygelfand/libcountertop/pkg/media/call"
	"sync"
)

type State = sharedcall.State
type Reason = sharedcall.Reason
type Call = sharedcall.Call
type View = sharedcall.View
type Profile = sharedcall.Profile
type Stats = sharedcall.Stats
type LayerStats = sharedcall.LayerStats
type Layout = sharedcall.Layout
type Calls struct{ *sharedcall.Calls }

func (c *Calls) Restore(v config.Config) { c.Calls.Restore(v.Call) }

const Idle = sharedcall.Idle
const Calling = sharedcall.Calling
const Ringing = sharedcall.Ringing
const Talking = sharedcall.Talking
const ReasonHangup = sharedcall.ReasonHangup
const ReasonDeclined = sharedcall.ReasonDeclined
const ReasonCancelled = sharedcall.ReasonCancelled
const ReasonUnanswered = sharedcall.ReasonUnanswered
const ReasonBusy = sharedcall.ReasonBusy
const ReasonUnavailable = sharedcall.ReasonUnavailable
const ReasonDropped = sharedcall.ReasonDropped
const ReasonFailed = sharedcall.ReasonFailed

var ErrBusy = sharedcall.ErrBusy
var ErrNoRoute = sharedcall.ErrNoRoute
var once sync.Once
var shared *Calls

func Get() *Calls {
	once.Do(func() {
		shared = &Calls{sharedcall.New(sharedcall.Options{Read: func() config.Call { return config.Get().Call }, Incoming: func(v bool) error { return config.Set().Call().Incoming(v) }, AutoAnswer: func(v bool) error { return config.Set().Call().AutoAnswer(v) }, PauseWake: func(v bool) error { return config.Set().Call().PauseWake(v) }, AutoVideo: func(v bool) error { return config.Set().Call().AutoVideo(v) }, Stream: func(v config.CallStream) error { return config.Set().Call().Stream(v) }, Self: discovery.Self, Find: discovery.Get().Find, Touch: discovery.Get().Touch, Port: web.Port, Shell: shell.Get(), Covered: privacy.Get().CameraCovered, Muted: privacy.Get().MicMuted, PrivacyChanged: func(f func()) func() { return privacy.Get().Changed.Listen(func(privacy.Marks) { f() }) }, PauseMedia: media.Get().Pause, Sounding: func() { volume.Get().Sounding(config.StreamVoice) }, Claim: func(name string, run func(context.Context) error) {
			speaker.Sound().Claim(name, func(ctx context.Context, _ *speaker.Speaker) error { return run(ctx) })
		}, Notice: notice, Audio: device{}, CameraStream: func() (int, sharedcall.Size, bool) { i, s, ok := cameraStream(); return i, sharedcall.Size(s), ok }, Frames: func(at int, sized func(sharedcall.Size)) (<-chan rtc.Frame, func()) {
			return camera{at: at, sized: func(s livecam.Size) { sized(sharedcall.Size(s)) }}.Frames()
		}, Key: func() { camera{}.Key() }, Helper: func() *surface.Client { return display.Get().Helper() }, Rotation: func() display.Orientation { return display.Get().Orientation() }, Native: func() (int, int) { return display.Get().Native() }, Ring: ring, Ringback: ringback})}
	})
	return shared
}
func init() {
	component.Register(component.Network, Get, component.Order(62))
	web.Handle("POST /call/offer", Get().OfferHandler())
	web.Handle("POST /call/answer", Get().AnswerHandler())
	web.Handle("POST /call/end", Get().EndHandler())
}
func SetIncoming(v bool)            { Get().SetIncoming(v) }
func SetAutoAnswer(v bool)          { Get().SetAutoAnswer(v) }
func SetPauseWake(v bool)           { Get().SetPauseWake(v) }
func SetAutoVideo(v bool)           { Get().SetAutoVideo(v) }
func SetStream(v config.CallStream) { Get().SetStream(v) }
func Arrange(w, h int) Layout       { return Get().Arrange(w, h) }
