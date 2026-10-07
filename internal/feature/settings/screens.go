package settings

import (
	"log/slog"
	"strings"

	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/a2dp"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/dhcp"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/network"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sendspin"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	"github.com/ygelfand/libcountertop/pkg/say"
)

// none is a row that does nothing when it is touched.
var none []func(int)

// open pushes a view, as what a chevron row does.
func open(v shell.View) func(int) { return func(int) { shell.Get().Push(v) } }

func root() *shell.Page {
	return sharedsettings.RootPage(sharedsettings.RootOptions{
		Network: func() string { return wifi.Get().Network() }, Theme: func() string { return themeSays(config.Get().Screen.Theme) }, Assistants: phrases, MediaLevel: func() int { return config.Get().Volume.Level(config.StreamMedia) }, Version: layout.Version, Push: shell.Get().Push,
		NetworkPage: func() shell.View { return networkPage() }, FeaturesPage: func() shell.View { return featuresPage() }, DisplayPage: func() shell.View { return displayPage() }, HomePage: func() shell.View { return homecontrol.Page() }, VolumePage: func() shell.View { return volume.Page() }, AssistantsPage: func() shell.View { return assistantsPage() }, SystemPage: func() shell.View { return systemPage() }, DebugPage: func() shell.View { return debugPage() }, AboutPage: func() shell.View { return aboutPage() },
		Sections: func() []sharedsettings.Section {
			return []sharedsettings.Section{{Label: "settings.camera", Glyph: gogui.IconCamera, Page: func() shell.View { return cameraPage() }}}
		},
	})
}

func phrases() string {
	var out []string
	for slot := range wakeword.Slots {
		if config.Get().Wake.Slot(slot).ID != "" {
			out = append(out, listening(slot))
		}
	}
	if len(out) == 0 {
		return say.T("assistants.off")
	}
	return strings.Join(out, ", ")
}

func debugPage() *shell.Page { return basicPages().Debug() }

var fpsIndex = sharedsettings.FpsIndex

var fpsLevel = sharedsettings.FpsLevel

var fpsSnap = sharedsettings.FpsSnap

var seedOf = sharedsettings.SeedOf

var seedLevel = sharedsettings.SeedLevel

var seedSays = sharedsettings.SeedSays

var fpsSays = sharedsettings.FpsSays

func displayPage() *shell.Page { return basicPages().Display() }

func sensorsPage() *shell.Page {
	return sharedsettings.PresencePage(sensors.Table, func() config.Presence { return config.Get().Presence }, sensors.SetPresence)
}

// clockPage is everything about the clock on the dashboard.
//
// Split out because Display had grown into a dozen rows of unrelated things — how bright the panel
// is, what the clock looks like, which side the dock hangs off — and a screen somebody scrolls to
// find one setting is a screen that has stopped being a menu.
// facePage picks how the clock is drawn, each row drawing the face it names.
//
// A list of words is the wrong picker for a look: "Cards" asks somebody to imagine it. A face
// already takes the box it is given, so the row hands it a small one and the option draws itself.
//
// One page for both faces, since the choice is the same one twice: which of these is showing, and
// which of these it settles into when nobody is there.
// visualPage picks the live audio visual.
func visualPage() *shell.Page {
	return visualPicker(say.T("debug.visuals"), "", nil, nil, func(k visual.Kind) {
		visuals.Get().SetKind(k)
		shell.Get().Push(visuals.Get().View())
	})
}

func visualPicker(title, empty string, none func(), chosen func() string, pick func(visual.Kind)) *shell.Page {
	return sharedsettings.VisualPicker(title, empty, none, chosen, pick, visual.ThumbnailFit)
}

func thumbTile(k visual.Kind) func(ui.Surface, ui.Rect, theme.Theme) {
	return sharedsettings.ThumbTile(k, visual.ThumbnailFit)
}

var posterSays = sharedsettings.PosterSays

func posterPage() *shell.Page { return basicPages().Poster() }

func everyPage() *shell.Page { return basicPages().PosterEvery() }

func usePosterEvery(e config.PosterEvery) func(int) {
	return func(int) { poster.Get().SetEvery(e) }
}

// preview draws one face at thumbnail size, on the surface it is sitting on rather than on the
// theme's background, so the row does not gain a panel the other rows do not have.
//
// Undated: the day under a clock the size of a postage stamp is a gray smear, and what the row is
// showing is the shape of the face rather than everything on it.
// positionPage picks where on the glass the clock sits, each row showing the box it would get.
// sizePage picks how much of its room the clock fills, each row showing what that leaves.
// colorPage picks what the clock is drawn in, each row drawing the clock in it.
//
// The face rather than a dot of the color. A dot says what the color is; the face says what the
// clock will look like, which is the question being asked — the same reason the faces draw
// themselves instead of being listed by name.
// placing and sizing are the two halves of the same preview: the clock as the dashboard would lay
// it out, with one of the two settings that decide that varied and the other left as it is.
//
// Left as it is rather than fixed, so the previews answer the question actually being asked. Small
// against Large means nothing without knowing where the clock sits, and a position page that always
// drew a full-size clock would be showing something the device is not about to do.
// laid draws the current face where these settings would put it, in a thumbnail the shape of the
// panel: both choices are about where on the screen the clock lands, so the preview has to show the
// whole screen rather than the part the clock would get.
// inking draws the current face in one of the colors on offer, filling the thumbnail: the question
// here is the color, so the clock is as large as the row allows rather than where it would sit.
func uiSize() string {
	if s := config.Get().Screen.Size; s != "" {
		return s
	}
	return board.Current().UISize
}

var sizeOf = sharedsettings.SizeOf

var sizeLevel = sharedsettings.SizeLevel

var sizeSnap = sharedsettings.SizeSnap

func setSize(name string) {
	if name == uiSize() {
		return
	}
	if err := screen.Get().Set("size", name); err != nil {
		slog.Error("the size could not be saved", "size", name, "err", err)
	}
}

func stylePage() *shell.Page { return basicPages().Style() }

func themePage() *shell.Page { return basicPages().Theme() }

// How a swatch is laid out, as fractions of its height.
const (
	swatchPad = 0.12

	// swatchGap is the space between the squares. Small: they are a set, and a gap wide enough to
	// separate them reads as four things rather than one palette.
	swatchGap = 0.07
)

// swatch draws a theme: its background as the card, with the rest as squares on it.
//
// The card carries the background rather than the background being one block among equals. It is
// the thing the eye reads first and the thing somebody is actually choosing between — whether the
// device will be light or dark — and as a fifth of a strip it was a detail. As the field the squares
// sit on it answers that before anything else is looked at, and it also puts each color against the
// background it will really be drawn on, which is the other half of picking a theme.
//
// Which colors is measured rather than picked. Averaging how far apart each role sits across the
// twelve themes, in RGB:
//
//	Background 197   Surface 197   Text 172   Accent 158   Accent2 115
//	Success 63   Warning 51   Muted 45   Danger 43
//
// There is a cliff after the first five. Success, Warning and Danger are the same traffic light in
// all twelve themes and Muted is the same gray, so drawing them makes most of every swatch identical
// to most of every other. A swatch is for telling themes apart, and a color they all share cannot do
// that. No filler to square the count off either: a block nobody can say why it is there is the same
// fault.
//
// The palette it is handed is the page's, not this row's, and is deliberately ignored: the point of
// the swatch is to show a theme that is not the one currently on.
var swatch = sharedsettings.Swatch

var themeSays = sharedsettings.ThemeSays

func use(name string) func(int) {
	return func(int) {
		if err := config.Set().Screen().Theme(name); err != nil {
			return
		}
		screen.Get().Use(name)
	}
}

// There was a Privacy page here, saying which way the two sliders were. It is gone: they are
// switches on the case, nothing on screen can move them, and a settings page that can only report
// is a page somebody opens once. The state is still an entity in Home Assistant and on the status
// page the web server serves, which is where something that cannot be changed from here belongs.

func networkPage() *shell.Page {
	return sharedsettings.NetworkPage(func() sharedsettings.NetworkState {
		c := sharedsettings.NetworkState{SSID: wifi.Get().Network(), Address: address(), MAC: wifi.Get().MAC(), Verify: config.Get().Network.Verify}
		if l := dhcp.Get().Lease(); l != nil {
			c.Router = l.Router.String()
			c.Renew = l.Renew
		}
		return c
	}, network.Get().SetVerify)
}

func callStreamPage() *shell.Page { return callPages().CallStream() }

func callsPage() *shell.Page { return callPages().Calls() }

func castPage() *shell.Page { return servicePages().Cast() }

func castAppsPage() *shell.Page { return servicePages().CastApps() }

func youtubePage() *shell.Page { return servicePages().YouTube() }

func sponsorPage() *shell.Page { return servicePages().Sponsor() }

func primePage() *shell.Page { return servicePages().Prime() }

// featuresPage is what the device does at all, as opposed to how it looks or how loud it is.
//
// These are whole subsystems rather than settings on one: each opens a port, holds hardware, or
// listens for something. They were switches in Home Assistant and nowhere else, which leaves a
// device that has not been adopted, or whose Home Assistant is down, unable to be set up from its
// own screen.
func featuresPage() *shell.Page {
	return sharedsettings.FeaturesPage(func() []sharedsettings.Feature {
		c := config.Get()
		return []sharedsettings.Feature{{Key: "features.sendspin", On: c.Sendspin.Enabled, Set: sendspin.Get().SetEnabled}, {Key: "features.cast", Page: func() shell.View { return castPage() }}, {Key: "call.settings", Page: func() shell.View { return callsPage() }}, {Key: "features.proxy", On: c.Bluetooth.Proxy, Set: bluetooth.Get().SetProxy}, {Key: "features.speaker", On: c.Bluetooth.Speaker, Set: a2dp.Get().SetEnabled}}
	}, shell.Get().Push)
}

func aboutPage() *shell.Page {
	return sharedsettings.AboutPage(func() sharedsettings.About {
		return sharedsettings.About{Name: config.Get().Device.Name, Version: layout.Version, Built: layout.BuildDate, Commit: layout.GitCommit}
	})
}

func address() string {
	l := dhcp.Get().Lease()
	if l == nil {
		return say.T("network.address.none")
	}
	return l.Address.IP.String()
}

// in is how long until something, for a time nobody wants as a date.
var in = sharedsettings.LeaseIn

// certificates says whether downloads are checked, on the page somebody looks at when one has
// failed. Not verifying is the answer that has to be plain rather than absent: it is a deliberate
// choice, and one nobody should discover by reading the source.
// zoneLabel is where the device thinks it is, or that it is taking the server's word for it.
func zoneLabel() string {
	if chosen := clock.Get().Zone(); chosen != "" {
		return chosen
	}
	return clock.FollowHome
}

// zonePage puts the device somewhere, whatever Home Assistant says it is.
//
// The list is short and written out rather than a zone database, because the device has no zone
// files: internal/lib/tz works from rules. Home Assistant is first, since following the server is
// what almost every device should do.
//
// A region at a time. A page holds about a dozen rows and does not scroll, so forty zones on one
// would put the last of them off the bottom of the screen where nothing can reach them.
func zonePage() *shell.Page { return basicPages().Zone() }

// chosenIn is the zone picked from a region, for the row that leads to it, so the current choice
// is visible without opening every region to find it.
var chosenIn = sharedsettings.ChosenIn

func regionPage(region string) *shell.Page { return basicPages().Region(region) }

func useZone(name string) func(int) {
	return func(int) { clock.Get().SetZone(name) }
}

// edgePage picks the side the dock comes in from. Home Assistant has the same setting, so a device
// mounted somewhere awkward can be fixed from either end.
func edgePage() *shell.Page { return basicPages().Dock() }

func useEdge(e config.Edge) func(int) {
	return func(int) { screen.Get().SetDrawer(e) }
}

// volumeEdgePage picks the side the volume card comes up on. Its own setting rather than the
// dock's: the dock's edge is where a hand swipes in from, and this is which side the volume keys
// are on. Left or right only, since the card is a column.
func volumeEdgePage() *shell.Page { return basicPages().VolumeEdge() }

func useVolumeEdge(e config.Edge) func(int) {
	return func(int) {
		if err := config.Set().Screen().Volume(e); err != nil {
			slog.Error("saving the volume edge failed", "err", err)
		}
	}
}

// Assistants are the wake word slots, which Home Assistant pairs with a pipeline each.
//
// Here as well as there for the reason the Features page exists: these were entities and nothing
// else, so a device whose Home Assistant is down, or which has not been adopted yet, could not be
// tuned from its own screen. Which wake word a slot listens for stays Home Assistant's, because it
// is the thing that hosts the models and pairs the pipeline — this is everything about how the
// slot behaves once it has fired.
func assistantsPage() *shell.Page { return assistantPages().Page() }

// listening is what a slot is set to hear, as the row's value.
func listening(slot int) string { return assistantPages().Listening(slot) }

// hearing says whether the slot can actually hear yet, which is not the same as being set: a model
// that Home Assistant named but the device has not got is a slot that looks armed and is deaf.
func hearing(slot int) string { return assistantPages().Hearing(slot) }

func assistantPage(slot int) *shell.Page { return assistantPages().Slot(slot) }

// A threshold runs from half to just under one: below half a model fires on anything, and at one
// nothing reaches it. The slider is the whole range, so its ends are those rather than nought and
// full.
const (
	leastSure = 50
	mostSure  = 99
)

var sensitivity = sharedsettings.Sensitivity

func setThreshold(slot int) func(int) { return assistantPages().Threshold(slot) }

func tonePage(slot int) *shell.Page { return assistantPages().Tone(slot) }

// useTone saves the tone and plays it, because what a tone sounds like is the whole of the choice
// and a list of four words is not it.
func useTone(slot int, c config.Chime) func(int) { return assistantPages().UseTone(slot, c) }

func deliveryPage(slot int) *shell.Page { return assistantPages().Delivery(slot) }

// delivering says what the choice costs, since neither option is better and the difference is not
// in the name.
var delivering = sharedsettings.Delivering

func useDelivery(slot int, d config.Delivery) func(int) { return assistantPages().UseDelivery(slot, d) }

// languagePage picks the text the panel shows.
//
// Only languages there are messages for are offered. A tag with nothing behind it settles on
// English anyway, so listing one would be offering a choice that does nothing.
func languagePage() *shell.Page { return basicPages().Language() }

func useLanguage(tag string) func(int) {
	return func(int) { SetLanguage(tag) }
}
