package settings

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/a2dp"
	"github.com/ygelfand/LANovo/internal/feature/access"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/feature/dhcp"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/network"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sendspin"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/wake"
	"github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	"github.com/ygelfand/LANovo/internal/ui/widget"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	"github.com/ygelfand/libcountertop/pkg/media/cast/protocols/youtube"
	"github.com/ygelfand/libcountertop/pkg/timezone"
)

// none is a row that does nothing when it is touched.
var none []func(int)

// open pushes a view, as what a chevron row does.
func open(v shell.View) func(int) { return func(int) { shell.Get().Push(v) } }

func root() *shell.Page {
	return &shell.Page{
		Title: say.T("settings.title"),
		Build: func() ([]widget.Row, []func(int)) {
			cfg := config.Get()

			return []widget.Row{
					{Glyph: gogui.IconGlobe, Label: say.T("settings.network"), Kind: widget.Chevron, Value: wifi.Get().Network()},
					{Glyph: gogui.IconPlug, Label: say.T("settings.features"), Kind: widget.Chevron},
					{Glyph: gogui.IconSunnyO, Label: say.T("settings.display"), Kind: widget.Chevron, Value: themeSays(cfg.Screen.Theme)},
					{Glyph: gogui.IconHome, Label: say.T("home.title"), Kind: widget.Chevron},
					{Glyph: gogui.IconSpeaker, Label: say.T("settings.volume"), Kind: widget.Chevron,
						Value: fmt.Sprintf("%d%%", cfg.Volume.Level(config.StreamMedia))},
					{Glyph: gogui.IconComment, Label: say.T("settings.assistants"), Kind: widget.Chevron, Value: phrases()},
					{Glyph: gogui.IconCamera, Label: say.T("settings.camera"), Kind: widget.Chevron},
					{Glyph: gogui.IconGear, Label: say.T("settings.system"), Kind: widget.Chevron},
					{Glyph: gogui.IconCode, Label: say.T("settings.debug"), Kind: widget.Chevron},
					{Glyph: gogui.IconInfo, Label: say.T("settings.about"), Kind: widget.Chevron, Value: layout.Version},
				}, []func(int){
					open(networkPage()),
					open(featuresPage()),
					open(displayPage()),
					open(homecontrol.Page()),
					open(volume.Page()),
					open(assistantsPage()),
					open(cameraPage()),
					open(systemPage()),
					open(debugPage()),
					open(aboutPage()),
				}
		},
	}
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

func debugPage() *shell.Page {
	return &shell.Page{
		Title: say.T("debug.title"),
		Build: func() ([]widget.Row, []func(int)) {
			fps, seed := config.Get().Visual.MaxFPS, config.Get().Visual.Seed
			adb := access.Get().ADB()
			return []widget.Row{
					{Glyph: gogui.IconTerminal, Label: say.T("system.adb"), Hint: say.T("system.adb.hint"), Kind: widget.Toggle, On: adb},
					{Label: say.T("debug.visuals"), Kind: widget.Chevron},
					{Label: say.T("debug.fps"), Kind: widget.Slider, Level: fpsLevel(fps), Snap: fpsSnap, Value: fpsSays(fps)},
					{Label: say.T("debug.seed"), Kind: widget.Slider, Level: seedLevel(seed), Value: seedSays(seed)},
				}, []func(int){
					func(int) { access.Get().SetADB(!adb) },
					open(visualPage()),
					func(level int) { visuals.Get().SetMaxFPS(config.MaxFPSSteps[fpsIndex(level)]) },
					func(level int) { visuals.Get().SetSeed(seedOf(level)) },
				}
		},
	}
}

func fpsIndex(level int) int {
	n := len(config.MaxFPSSteps) - 1
	return min(max((level*n+50)/100, 0), n)
}

func fpsLevel(fps int) int {
	i := max(slices.Index(config.MaxFPSSteps, fps), 0)
	return i * 100 / (len(config.MaxFPSSteps) - 1)
}

func fpsSnap(level int) int { return fpsLevel(config.MaxFPSSteps[fpsIndex(level)]) }

func seedOf(level int) int {
	if level <= 0 {
		return 0
	}
	return max(1, (min(level, 100)*visual.SeedMost+50)/100)
}

func seedLevel(seed int) int {
	if seed <= 0 {
		return 0
	}
	return max(1, (seed*100+visual.SeedMost/2)/visual.SeedMost)
}

func seedSays(seed int) string {
	if seed <= 0 {
		return say.T("debug.seed.random")
	}
	return say.F("debug.seed.value", map[string]any{"N": seed})
}

func fpsSays(fps int) string {
	return say.F("debug.fps.value", map[string]any{"N": fps})
}

func displayPage() *shell.Page {
	return &shell.Page{
		Title: say.T("display.title"),
		Build: func() ([]widget.Row, []func(int)) {
			cfg := config.Get()
			auto := cfg.Screen.Mode == config.ModeAuto

			return []widget.Row{
					{Label: say.T("display.brightness"), Kind: widget.Slider, Level: cfg.Screen.Backlight},
					{Label: say.T("display.auto"), Kind: widget.Toggle, On: auto},
					{Glyph: gogui.IconPalette, Label: say.T("display.theme"), Kind: widget.Chevron, Value: themeSays(cfg.Screen.Theme)},
					{Label: say.T("display.style"), Kind: widget.Chevron, Value: say.T("style." + style.ByName(cfg.Screen.Style).Name)},
					{Label: say.T("display.size"), Kind: widget.Slider, Level: sizeLevel(uiSize()), Snap: sizeSnap, Value: say.T("uisize." + uiSize())},
					{Glyph: gogui.IconClock, Label: say.T("display.clock"), Kind: widget.Chevron, Value: cfg.Clock.Face.Label()},
					{Glyph: gogui.IconMoon, Label: say.T("display.idle"), Kind: widget.Chevron, Value: cfg.Idle.After.Label()},
					{Glyph: gogui.IconPicture, Label: say.T("display.poster"), Kind: widget.Chevron, Value: posterSays(cfg.Poster)},
					{Label: say.T("controls.dock"), Kind: widget.Chevron, Value: cfg.Screen.Drawer.Label()},
					{Label: say.T("controls.volume"), Kind: widget.Chevron, Value: cfg.Screen.Volume.Label()},
					{Glyph: gogui.IconEye, Label: say.T("display.presence"), Kind: widget.Chevron},
				}, []func(int){
					setBrightness,
					func(int) { toggleAuto(auto) },
					open(themePage()),
					open(stylePage()),
					func(level int) { setSize(sizeOf(level)) },
					open(clockPage()),
					open(idlePage()),
					open(posterPage()),
					open(edgePage()),
					open(volumeEdgePage()),
					open(sensorsPage()),
				}
		},
	}
}

func sensorsPage() *shell.Page {
	return &shell.Page{
		Title: say.T("display.presence"),
		Build: func() ([]widget.Row, []func(int)) {
			cfg := config.Get()
			p := cfg.Presence
			wake, reach := sensors.Table().Row("wake"), sensors.Table().Row("range")
			savePresence := func(name, v string) {
				if err := sensors.SetPresence(name, v); err != nil {
					slog.Error("the presence setting could not be saved", "setting", name, "err", err)
				}
			}
			return []widget.Row{
					{Label: say.T("display.marks"), Kind: widget.Toggle, On: cfg.Screen.Marks},
					{Label: wake.Title(), Kind: widget.Toggle, On: wake.On(&p)},
					{Label: reach.Title(), Kind: widget.Slider, Level: reach.Percent(p.Range), Value: reach.Read(&p),
						Snap: func(level int) int { return reach.Percent(reach.Raw(level)) }},
				}, []func(int){
					func(int) { privacy.Get().SetMarks(!cfg.Screen.Marks) },
					func(int) { savePresence("wake", setting.OnOff(!p.Wake)) },
					func(level int) { savePresence("range", strconv.Itoa(reach.Raw(level))) },
				}
		},
	}
}

// clockPage is everything about the clock on the dashboard.
//
// Split out because Display had grown into a dozen rows of unrelated things — how bright the panel
// is, what the clock looks like, which side the dock hangs off — and a screen somebody scrolls to
// find one setting is a screen that has stopped being a menu.
func clockPage() *shell.Page {
	return &shell.Page{
		Title: say.T("clock.title"),
		Build: func() ([]widget.Row, []func(int)) {
			cfg := config.Get()

			return []widget.Row{
					{Label: say.T("clock.face"), Kind: widget.Chevron, Value: cfg.Clock.Face.Label()},
					{Label: say.T("clock.position"), Kind: widget.Chevron,
						Value: cfg.Clock.Position.Label()},
					{Label: say.T("clock.size"), Kind: widget.Chevron, Value: cfg.Clock.Size.Label()},
					{Label: say.T("clock.color"), Kind: widget.Chevron, Value: cfg.Clock.Ink.Label()},
					{Label: say.T("clock.date"), Kind: widget.Toggle, On: cfg.Clock.Date},
				}, []func(int){
					open(facePage(say.T("clock.face"),
						func() config.Face { return config.Get().Clock.Face },
						dashboard.Get().SetFace)),
					open(positionPage()),
					open(sizePage()),
					open(colorPage()),
					func(int) { dashboard.Get().SetDate(!cfg.Clock.Date) },
				}
		},
	}
}

// facePage picks how the clock is drawn, each row drawing the face it names.
//
// A list of words is the wrong picker for a look: "Cards" asks somebody to imagine it. A face
// already takes the box it is given, so the row hands it a small one and the option draws itself.
//
// One page for both faces, since the choice is the same one twice: which of these is showing, and
// which of these it settles into when nobody is there.
func facePage(title string, now func() config.Face, use func(config.Face)) *shell.Page {
	return &shell.Page{
		Title: title,
		Tiles: func() ([]widget.Cell, []func(int)) {
			chosen := now()

			cells := make([]widget.Cell, 0, len(config.Faces()))
			acts := make([]func(int), 0, len(config.Faces()))

			for _, f := range config.Faces() {
				cells = append(cells, widget.Cell{
					Label:  f.Label(),
					Chosen: f == chosen,
					Paint:  preview(f),
					Face:   f,
				})
				acts = append(acts, picks(use, f))
			}
			return cells, acts
		},
	}
}

func picks(use func(config.Face), f config.Face) func(int) {
	return func(int) { use(f) }
}

// visualPage picks the live audio visual.
func visualPage() *shell.Page {
	return visualPicker(say.T("debug.visuals"), "", nil, nil, func(k visual.Kind) {
		visuals.Get().SetKind(k)
		shell.Get().Push(visuals.Get().View())
	})
}

func visualPicker(title, empty string, none func(), chosen func() string, pick func(visual.Kind)) *shell.Page {
	return &shell.Page{
		Title: title,
		Tiles: func() ([]widget.Cell, []func(int)) {
			now := ""
			if chosen != nil {
				now = chosen()
			}
			var cells []widget.Cell
			var acts []func(int)
			if none != nil {
				cells = append(cells, widget.Cell{Label: empty, Chosen: chosen != nil && now == "", Paint: noneTile})
				acts = append(acts, func(int) { none() })
			}
			for _, k := range visual.Built() {
				cells = append(cells, widget.Cell{Label: k.Label(), Chosen: chosen != nil && string(k) == now, Paint: thumbTile(k)})
				acts = append(acts, func(int) { pick(k) })
			}
			return cells, acts
		},
	}
}

func thumbTile(k visual.Kind) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, at ui.Rect, palette theme.Theme) {
		img := visual.ThumbnailFit(k, at.W, at.H)
		if img == nil {
			ui.FillRect(s, at, palette.Surface)
			return
		}
		ui.DrawRGBA(s, at.X, at.Y, img, 1, ui.ClipOf(s))
	}
}

func posterSays(p config.Poster) string {
	if !p.Enabled {
		return say.T("poster.off")
	}
	return p.Every.Label()
}

func posterPage() *shell.Page {
	return &shell.Page{
		Title: say.T("display.poster"),
		Build: func() ([]widget.Row, []func(int)) {
			cfg := config.Get().Poster
			return []widget.Row{
					{Label: say.T("poster.show"), Kind: widget.Toggle, On: cfg.Enabled},
					{Label: say.T("poster.next"), Dim: !cfg.Enabled},
					{Label: say.T("poster.every"), Kind: widget.Chevron, Value: cfg.Every.Label(), Dim: !cfg.Enabled},
				}, []func(int){
					func(int) { poster.Get().SetEnabled(!cfg.Enabled) },
					func(int) { poster.Get().Next() },
					open(everyPage()),
				}
		},
	}
}

func everyPage() *shell.Page {
	return &shell.Page{
		Title: say.T("poster.every"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Poster.Every
			var rows []widget.Row
			var acts []func(int)
			for _, e := range config.PosterEveries() {
				rows = append(rows, widget.Row{Label: e.Label(), Chosen: e == now})
				acts = append(acts, usePosterEvery(e))
			}
			return rows, acts
		},
	}
}

func usePosterEvery(e config.PosterEvery) func(int) {
	return func(int) { poster.Get().SetEvery(e) }
}

// preview draws one face at thumbnail size, on the surface it is sitting on rather than on the
// theme's background, so the row does not gain a panel the other rows do not have.
//
// Undated: the day under a clock the size of a postage stamp is a gray smear, and what the row is
// showing is the shape of the face rather than everything on it.
func preview(f config.Face) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, at ui.Rect, palette theme.Theme) {
		cfg := config.Get()
		reading := face.Read(time.Now(), cfg.Screen.Hours == config.TwentyFourHour)

		// In the clock's own color, not the theme's. The row is previewing this device's clock, and
		// a list of gray thumbnails picking a shape for an amber clock is previewing a different one.
		face.Of(f).Draw(s, at, reading.Undated(), cfg.Clock.Ink.Over(palette))
	}
}

// positionPage picks where on the glass the clock sits, each row showing the box it would get.
func positionPage() *shell.Page {
	return &shell.Page{
		Title: say.T("clock.position.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Clock.Position

			rows := make([]widget.Row, 0, len(config.Positions()))
			acts := make([]func(int), 0, len(config.Positions()))

			for _, at := range config.Positions() {
				rows = append(rows, widget.Row{
					Label:   at.Label(),
					Chosen:  at == now,
					Preview: placing(at),
				})
				acts = append(acts, usePosition(at))
			}
			return rows, acts
		},
	}
}

func usePosition(at config.Position) func(int) {
	return func(int) { dashboard.Get().SetPosition(at) }
}

// sizePage picks how much of its room the clock fills, each row showing what that leaves.
func sizePage() *shell.Page {
	return &shell.Page{
		Title: say.T("clock.size.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Clock.Size

			rows := make([]widget.Row, 0, len(config.Sizes()))
			acts := make([]func(int), 0, len(config.Sizes()))

			for _, size := range config.Sizes() {
				rows = append(rows, widget.Row{
					Label:   size.Label(),
					Chosen:  size == now,
					Preview: sizing(size),
				})
				acts = append(acts, useSize(size))
			}
			return rows, acts
		},
	}
}

func useSize(size config.Size) func(int) {
	return func(int) { dashboard.Get().SetSize(size) }
}

// colorPage picks what the clock is drawn in, each row drawing the clock in it.
//
// The face rather than a dot of the color. A dot says what the color is; the face says what the
// clock will look like, which is the question being asked — the same reason the faces draw
// themselves instead of being listed by name.
func colorPage() *shell.Page {
	return &shell.Page{
		Title: say.T("clock.color.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Clock.Ink

			rows := make([]widget.Row, 0, len(config.Inks()))
			acts := make([]func(int), 0, len(config.Inks()))

			for _, ink := range config.Inks() {
				rows = append(rows, widget.Row{
					Label:   ink.Label(),
					Chosen:  ink == now,
					Preview: inking(ink),
				})
				acts = append(acts, useInk(ink))
			}
			return rows, acts
		},
	}
}

func useInk(ink config.Ink) func(int) {
	return func(int) { dashboard.Get().SetInk(ink) }
}

// placing and sizing are the two halves of the same preview: the clock as the dashboard would lay
// it out, with one of the two settings that decide that varied and the other left as it is.
//
// Left as it is rather than fixed, so the previews answer the question actually being asked. Small
// against Large means nothing without knowing where the clock sits, and a position page that always
// drew a full-size clock would be showing something the device is not about to do.
func placing(at config.Position) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, box ui.Rect, palette theme.Theme) {
		laid(s, box, palette, at, config.Get().Clock.Size)
	}
}

func sizing(size config.Size) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, box ui.Rect, palette theme.Theme) {
		laid(s, box, palette, config.Get().Clock.Position, size)
	}
}

// laid draws the current face where these settings would put it, in a thumbnail the shape of the
// panel: both choices are about where on the screen the clock lands, so the preview has to show the
// whole screen rather than the part the clock would get.
func laid(s ui.Surface, box ui.Rect, palette theme.Theme, at config.Position, size config.Size) {
	cfg := config.Get()
	reading := face.Read(time.Now(), cfg.Screen.Hours == config.TwentyFourHour)

	within := dashboard.Box(at, size, box.W, box.H)
	within.X += box.X
	within.Y += box.Y

	face.Of(cfg.Clock.Face).Draw(s, within, reading.Undated(), cfg.Clock.Ink.Over(palette))
}

// inking draws the current face in one of the colors on offer, filling the thumbnail: the question
// here is the color, so the clock is as large as the row allows rather than where it would sit.
func inking(ink config.Ink) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, at ui.Rect, palette theme.Theme) {
		cfg := config.Get()
		reading := face.Read(time.Now(), cfg.Screen.Hours == config.TwentyFourHour)

		face.Of(cfg.Clock.Face).Draw(s, at, reading.Undated(), ink.Over(palette))
	}
}

func uiSize() string {
	if s := config.Get().Screen.Size; s != "" {
		return s
	}
	return board.Current().UISize
}

func sizeOf(level int) string {
	all := config.ScreenSizes()
	last := len(all) - 1
	return all[min(max((level*last+50)/100, 0), last)]
}

func sizeLevel(name string) int {
	all := config.ScreenSizes()
	return max(slices.Index(all, name), 0) * 100 / (len(all) - 1)
}

func sizeSnap(level int) int { return sizeLevel(sizeOf(level)) }

func setSize(name string) {
	if name == uiSize() {
		return
	}
	if err := screen.Get().Set("size", name); err != nil {
		slog.Error("the size could not be saved", "size", name, "err", err)
	}
}

func stylePage() *shell.Page {
	return &shell.Page{
		Title: say.T("style.title"),
		Tiles: func() ([]widget.Cell, []func(int)) {
			now := style.ByName(config.Get().Screen.Style).Name
			var cells []widget.Cell
			var taps []func(int)
			for _, name := range style.Names() {
				cells = append(cells, widget.Cell{Label: say.T("style." + name), Chosen: name == now, Style: name})
				taps = append(taps, func(int) {
					if err := screen.Get().Set("style", name); err != nil {
						slog.Error("the style could not be saved", "style", name, "err", err)
					}
				})
			}
			return cells, taps
		},
	}
}

func themePage() *shell.Page {
	return &shell.Page{
		Title: say.T("theme.title"),

		// A grid rather than a row each: a row spends its width on a label and a small swatch, and
		// twelve of them do not fit on a screen. A tile is the swatch, with the name on it.
		Tiles: func() ([]widget.Cell, []func(int)) {
			now := config.Get().Screen.Theme
			follow, _ := theme.ByName(style.Theme(config.Get().Screen.Style, style.ThemeDefault))

			cells := []widget.Cell{{Label: say.T("theme.default"), Chosen: now == style.ThemeDefault, Palette: &follow}}
			acts := []func(int){use(style.ThemeDefault)}

			for _, t := range theme.All {
				cells = append(cells, widget.Cell{
					Label:   t.Name,
					Chosen:  t.Name == now,
					Paint:   swatch(t),
					Palette: &t,
				})
				acts = append(acts, use(t.Name))
			}
			return cells, acts
		},
	}
}

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
func swatch(t theme.Theme) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, at ui.Rect, _ theme.Theme) {
		ui.FillRect(s, at, t.Background)

		// A hairline, because a light theme's card on a light page has no edge of its own — and the
		// card being that color is the first thing the row is there to say.
		ui.Border(s, at, max(at.H/40, 1), t.Text.Blend(t.Background, 0.7))

		on := []theme.Color{t.Surface, t.Text, t.Accent, t.Accent2}

		pad := int(float64(at.H) * swatchPad)
		gap := int(float64(at.H) * swatchGap)

		// Square, so the side is whichever of the two runs out first: the height the card leaves, or
		// what is left of the width once the gaps are taken out.
		wide := (at.W - pad*2 - gap*(len(on)-1)) / len(on)
		side := min(at.H-pad*2, wide)

		// Centered as a block rather than spread to the edges, so the squares stay a group on the
		// card instead of four things pinned to its corners.
		x := at.X + (at.W-side*len(on)-gap*(len(on)-1))/2
		y := at.Y + (at.H-side)/2

		for i, c := range on {
			ui.FillRect(s, ui.Rect{X: x + i*(side+gap), Y: y, W: side, H: side}, c)
		}
	}
}

func themeSays(name string) string {
	if name == style.ThemeDefault {
		return say.T("theme.default")
	}
	return name
}

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
	return &shell.Page{
		Title: say.T("network.title"),
		Build: func() ([]widget.Row, []func(int)) {
			verify := config.Get().Network.Verify
			rows := []widget.Row{
				{Label: say.T("network.ssid"), Kind: widget.Plain, Value: wifi.Get().Network()},
				{Label: say.T("network.address"), Kind: widget.Plain, Value: address()},
				{Label: say.T("network.mac"), Kind: widget.Plain, Value: wifi.Get().MAC()},
			}
			acts := []func(int){nil, nil, nil}
			if l := dhcp.Get().Lease(); l != nil {
				rows = append(rows,
					widget.Row{Label: say.T("network.router"), Kind: widget.Plain, Value: l.Router.String()},
					widget.Row{Label: say.T("network.lease"), Kind: widget.Plain, Value: in(l.Renew)})
				acts = append(acts, nil, nil)
			}
			rows = append(rows, widget.Row{Glyph: gogui.IconLock, Label: say.T("system.certificates"),
				Hint: say.T("system.certificates.hint"), Kind: widget.Toggle, On: verify})
			acts = append(acts, func(int) { network.Get().SetVerify(!verify) })
			return rows, acts
		},
	}
}

func callStreamPage() *shell.Page {
	return &shell.Page{
		Title: say.T("call.stream"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Call.Stream
			var rows []widget.Row
			var acts []func(int)
			for _, v := range config.CallStreams() {
				rows = append(rows, widget.Row{Label: v.Label(), Kind: widget.Plain, Chosen: v == now})
				acts = append(acts, func(int) { call.SetStream(v) })
			}
			return rows, acts
		},
	}
}

func callsPage() *shell.Page {
	return &shell.Page{
		Title: say.T("call.settings"),
		Build: func() ([]widget.Row, []func(int)) {
			c := config.Get().Call
			rows := []widget.Row{
				{Label: say.T("call.incoming.allow"), Kind: widget.Toggle, On: c.Incoming},
				{Label: say.T("call.auto"), Hint: say.T("call.auto.hint"), Kind: widget.Toggle, On: c.AutoAnswer},
			}
			acts := []func(int){
				func(int) { call.SetIncoming(!c.Incoming) },
				func(int) { call.SetAutoAnswer(!c.AutoAnswer) },
			}
			if c.AutoAnswer {
				rows = append(rows, widget.Row{Label: say.T("call.autovideo"), Kind: widget.Toggle, On: c.AutoVideo})
				acts = append(acts, func(int) { call.SetAutoVideo(!c.AutoVideo) })
			}
			rows = append(rows, widget.Row{Label: say.T("call.pausewake"), Kind: widget.Toggle, On: c.PauseWake})
			acts = append(acts, func(int) { call.SetPauseWake(!c.PauseWake) })
			rows = append(rows, widget.Row{Label: say.T("call.stream"), Hint: say.T("call.stream.hint"), Kind: widget.Chevron, Value: c.Stream.Label()})
			acts = append(acts, open(callStreamPage()))
			return rows, acts
		},
	}
}

func castPage() *shell.Page {
	return &shell.Page{
		Title: say.T("features.cast"),
		Build: func() ([]widget.Row, []func(int)) {
			on := config.Get().Cast.Receiver
			return []widget.Row{
					{Label: say.T("cast.enable"), Kind: widget.Toggle, On: on},
					{Label: say.T("cast.apps"), Kind: widget.Chevron},
				}, []func(int){
					func(int) { chromecast.Get().SetReceiver(!on) },
					open(castAppsPage()),
				}
		},
	}
}

func castAppsPage() *shell.Page {
	return &shell.Page{
		Title: say.T("cast.apps"),
		Build: func() ([]widget.Row, []func(int)) {
			return []widget.Row{
					{Label: say.T("youtube.title"), Kind: widget.Chevron},
					{Label: say.T("cast.prime"), Kind: widget.Chevron},
				}, []func(int){
					open(youtubePage()),
					open(primePage()),
				}
		},
	}
}

func youtubePage() *shell.Page {
	return &shell.Page{
		Title: say.T("youtube.title"),
		Build: func() ([]widget.Row, []func(int)) {
			return []widget.Row{
					{Label: say.T("youtube.skip"), Kind: widget.Chevron, Value: strconv.Itoa(len(config.Get().Cast.YouTube.Skip))},
				}, []func(int){
					open(sponsorPage()),
				}
		},
	}
}

func sponsorPage() *shell.Page {
	return &shell.Page{
		Title: say.T("youtube.skip"),
		Build: func() ([]widget.Row, []func(int)) {
			chosen := config.Get().Cast.YouTube.Skip
			rows := make([]widget.Row, 0, len(youtube.Categories))
			acts := make([]func(int), 0, len(youtube.Categories))
			for _, c := range youtube.Categories {
				on := slices.Contains(chosen, c)
				rows = append(rows, widget.Row{Label: say.T("youtube.category." + c), Kind: widget.Toggle, On: on})
				acts = append(acts, func(int) {
					next := slices.DeleteFunc(slices.Clone(chosen), func(x string) bool { return x == c })
					if !on {
						next = append(next, c)
					}
					chromecast.Get().SetSkip(strings.Join(next, ","))
				})
			}
			return rows, acts
		},
	}
}

func primePage() *shell.Page {
	return &shell.Page{
		Title: say.T("cast.prime"),
		Build: func() ([]widget.Row, []func(int)) {
			persist, skip := config.Get().Cast.Prime.Persist, config.Get().Cast.Prime.SkipIntro
			state := say.T("cast.prime.unregistered")
			if chromecast.Get().PrimeRegistered() {
				state = say.T("cast.prime.registered")
			}
			return []widget.Row{
					{Label: say.T("cast.prime.persist"), Hint: say.T("cast.prime.persist.hint"), Kind: widget.Toggle, On: persist},
					{Label: say.T("cast.prime.reset"), Kind: widget.Plain, Value: state},
					{Label: say.T("cast.prime.skipintro"), Hint: say.T("cast.prime.skipintro.hint"), Kind: widget.Toggle, On: skip},
				}, []func(int){
					func(int) { chromecast.Get().SetPrimePersist(!persist) },
					func(int) { chromecast.Get().ResetPrime() },
					func(int) { chromecast.Get().SetPrimeSkipIntro(!skip) },
				}
		},
	}
}

// featuresPage is what the device does at all, as opposed to how it looks or how loud it is.
//
// These are whole subsystems rather than settings on one: each opens a port, holds hardware, or
// listens for something. They were switches in Home Assistant and nowhere else, which leaves a
// device that has not been adopted, or whose Home Assistant is down, unable to be set up from its
// own screen.
func featuresPage() *shell.Page {
	return &shell.Page{
		Title: say.T("features.title"),
		Build: func() ([]widget.Row, []func(int)) {
			cfg := config.Get()

			return []widget.Row{
					{
						Label: say.T("features.sendspin"),
						Hint:  say.T("features.sendspin.hint"),
						Kind:  widget.Toggle,
						On:    cfg.Sendspin.Enabled,
					},
					{
						Label: say.T("features.cast"),
						Hint:  say.T("features.cast.hint"),
						Kind:  widget.Chevron,
					},
					{
						Label: say.T("call.settings"),
						Hint:  say.T("call.settings.hint"),
						Kind:  widget.Chevron,
					},
					{
						Label: say.T("features.proxy"),
						Hint:  say.T("features.proxy.hint"),
						Kind:  widget.Toggle,
						On:    cfg.Bluetooth.Proxy,
					},
					{
						Label: say.T("features.speaker"),
						Hint:  say.T("features.speaker.hint"),
						Kind:  widget.Toggle,
						On:    cfg.Bluetooth.Speaker,
					},
				}, []func(int){
					func(int) { sendspin.Get().SetEnabled(!cfg.Sendspin.Enabled) },
					open(castPage()),
					open(callsPage()),
					func(int) { bluetooth.Get().SetProxy(!cfg.Bluetooth.Proxy) },
					func(int) { a2dp.Get().SetEnabled(!cfg.Bluetooth.Speaker) },
				}
		},
	}
}

func aboutPage() *shell.Page {
	return &shell.Page{
		Title: say.T("about.title"),
		Build: func() ([]widget.Row, []func(int)) {
			return []widget.Row{
				{Label: say.T("about.name"), Kind: widget.Plain, Value: config.Get().Device.Name},
				{Label: say.T("about.version"), Kind: widget.Plain, Value: layout.Version},
				{Label: say.T("about.built"), Kind: widget.Plain, Value: layout.BuildDate},
				{Label: say.T("about.commit"), Kind: widget.Plain, Value: layout.GitCommit},
			}, none
		},
	}
}

func address() string {
	l := dhcp.Get().Lease()
	if l == nil {
		return say.T("network.address.none")
	}
	return l.Address.IP.String()
}

// in is how long until something, for a time nobody wants as a date.
func in(at time.Time) string {
	d := time.Until(at)
	if d <= 0 {
		return say.T("network.lease.now")
	}
	if d < time.Hour {
		return say.F("network.lease.minutes", map[string]any{"N": int(d.Minutes())})
	}
	return say.F("network.lease.hours", map[string]any{"N": fmt.Sprintf("%.1f", d.Hours())})
}

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
func zonePage() *shell.Page {
	return &shell.Page{
		Title: say.T("zone.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := clock.Get().Zone()

			rows := []widget.Row{{Label: clock.FollowHome, Chosen: now == ""}}
			acts := []func(int){useZone("")}

			// Zones that are not in a region stand on their own, which today is UTC.
			for _, z := range tz.In("") {
				rows = append(rows, widget.Row{Label: z.Name, Chosen: z.Name == now})
				acts = append(acts, useZone(z.Name))
			}

			for _, region := range tz.Regions() {
				rows = append(rows, widget.Row{
					Label: region, Kind: widget.Chevron, Value: chosenIn(region, now),
				})
				acts = append(acts, open(regionPage(region)))
			}
			return rows, acts
		},
	}
}

// chosenIn is the zone picked from a region, for the row that leads to it, so the current choice
// is visible without opening every region to find it.
func chosenIn(region, now string) string {
	if now != "" && tz.Region(now) == region {
		return now[len(region)+1:]
	}
	return ""
}

func regionPage(region string) *shell.Page {
	return &shell.Page{
		Title: region,
		Build: func() ([]widget.Row, []func(int)) {
			now := clock.Get().Zone()

			var (
				rows []widget.Row
				acts []func(int)
			)
			for _, z := range tz.In(region) {
				rows = append(rows, widget.Row{
					Label: z.Name[len(region)+1:], Chosen: z.Name == now,
				})
				acts = append(acts, useZone(z.Name))
			}
			return rows, acts
		},
	}
}

func useZone(name string) func(int) {
	return func(int) { clock.Get().SetZone(name) }
}

// edgePage picks the side the dock comes in from. Home Assistant has the same setting, so a device
// mounted somewhere awkward can be fixed from either end.
func edgePage() *shell.Page {
	return &shell.Page{
		Title: say.T("dock.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Screen.Drawer

			var (
				rows []widget.Row
				acts []func(int)
			)
			for _, e := range config.Edges() {
				rows = append(rows, widget.Row{Label: e.Label(), Chosen: e == now})
				acts = append(acts, useEdge(e))
			}
			return rows, acts
		},
	}
}

func useEdge(e config.Edge) func(int) {
	return func(int) { screen.Get().SetDrawer(e) }
}

// volumeEdgePage picks the side the volume card comes up on. Its own setting rather than the
// dock's: the dock's edge is where a hand swipes in from, and this is which side the volume keys
// are on. Left or right only, since the card is a column.
func volumeEdgePage() *shell.Page {
	return &shell.Page{
		Title: say.T("volume.location.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Screen.Volume

			var (
				rows []widget.Row
				acts []func(int)
			)
			for _, e := range config.Sides() {
				rows = append(rows, widget.Row{Label: e.Label(), Chosen: e == now})
				acts = append(acts, useVolumeEdge(e))
			}
			return rows, acts
		},
	}
}

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
func assistantsPage() *shell.Page {
	return &shell.Page{
		Title: say.T("assistants.title"),
		Build: func() ([]widget.Row, []func(int)) {
			var (
				rows []widget.Row
				acts []func(int)
			)
			for slot := range wakeword.Slots {
				rows = append(rows, widget.Row{
					Label: say.F("assistants.slot", map[string]any{"N": slot + 1}),
					Kind:  widget.Chevron,
					Value: listening(slot),

					Hint:   hearing(slot),
					Chosen: false,
				})
				acts = append(acts, open(assistantPage(slot)))
			}
			return rows, acts
		},
	}
}

// listening is what a slot is set to hear, as the row's value.
func listening(slot int) string {
	if id := config.Get().Wake.Slot(slot).ID; id != "" {
		return wake.Pick(wake.Lib().Ours(), id).Phrase
	}
	return say.T("assistants.off")
}

// hearing says whether the slot can actually hear yet, which is not the same as being set: a model
// that Home Assistant named but the device has not got is a slot that looks armed and is deaf.
func hearing(slot int) string {
	id := config.Get().Wake.Slot(slot).ID
	if id == "" {
		return say.T("assistants.unset")
	}
	if _, ok := wake.Find(wake.Lib().Ours(), id); !ok {
		return say.T("assistants.waiting")
	}
	return ""
}

func assistantPage(slot int) *shell.Page {
	return &shell.Page{
		Title: say.F("assistants.slot", map[string]any{"N": slot + 1}),
		Build: func() ([]widget.Row, []func(int)) {
			saved := config.Get().Wake.Slot(slot)

			return []widget.Row{
					// First, because it is the one row that does something rather than setting
					// something: a way to talk to this assistant without saying its word.
					{Label: say.T("assistants.talk"), Hint: say.T("assistants.talk.hint")},

					{Label: say.T("assistants.threshold"), Kind: widget.Slider, Level: sensitivity(saved.Threshold),
						Hint: say.T("assistants.threshold.hint")},

					{Label: say.T("assistants.tone"), Kind: widget.Chevron, Value: saved.Tone.Label()},
					{Label: say.T("assistants.reply"), Kind: widget.Chevron, Value: saved.Delivery.Label()},
					{Label: say.T("look.title"), Kind: widget.Chevron, Value: saved.Look.Place.Label()},
				}, []func(int){
					func(int) { wakeword.Requested.Emit(slot) },
					setThreshold(slot),
					open(tonePage(slot)),
					open(deliveryPage(slot)),
					open(lookPage(slot)),
				}
		},
	}
}

// A threshold runs from half to just under one: below half a model fires on anything, and at one
// nothing reaches it. The slider is the whole range, so its ends are those rather than nought and
// full.
const (
	leastSure = 50
	mostSure  = 99
)

func sensitivity(v float64) int {
	return leastSure + mostSure - min(max(int(v*100+0.5), leastSure), mostSure)
}

func setThreshold(slot int) func(int) {
	return func(level int) {
		v := float64(leastSure+mostSure-min(max(level, leastSure), mostSure)) / 100
		if err := config.Set().Wake(slot).Threshold(v); err != nil {
			slog.Error("saving the wake threshold failed", "slot", slot+1, "err", err)
		}
	}
}

func tonePage(slot int) *shell.Page {
	return &shell.Page{
		Title: say.T("tone.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Wake.Slot(slot).Tone

			var (
				rows []widget.Row
				acts []func(int)
			)
			for _, c := range config.Chimes() {
				rows = append(rows, widget.Row{Label: c.Label(), Chosen: c == now})
				acts = append(acts, useTone(slot, c))
			}
			return rows, acts
		},
	}
}

// useTone saves the tone and plays it, because what a tone sounds like is the whole of the choice
// and a list of four words is not it.
func useTone(slot int, c config.Chime) func(int) {
	return func(int) {
		if err := config.Set().Wake(slot).Tone(c); err != nil {
			slog.Error("saving the wake tone failed", "slot", slot+1, "err", err)
			return
		}
		wakeword.Chime(slot)
	}
}

func deliveryPage(slot int) *shell.Page {
	return &shell.Page{
		Title: say.T("reply.title"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Wake.Slot(slot).Delivery

			var (
				rows []widget.Row
				acts []func(int)
			)
			for _, d := range config.Deliveries() {
				rows = append(rows, widget.Row{
					Label: d.Label(), Chosen: d == now, Hint: delivering(d),
				})
				acts = append(acts, useDelivery(slot, d))
			}
			return rows, acts
		},
	}
}

// delivering says what the choice costs, since neither option is better and the difference is not
// in the name.
func delivering(d config.Delivery) string {
	if d == config.DeliveryStream {
		return say.T("reply.stream")
	}
	return say.T("reply.download")
}

func useDelivery(slot int, d config.Delivery) func(int) {
	return func(int) {
		if err := config.Set().Wake(slot).Delivery(d); err != nil {
			slog.Error("saving the reply delivery failed", "slot", slot+1, "err", err)
		}
	}
}

// languagePage picks the text the panel shows.
//
// Only languages there are messages for are offered. A tag with nothing behind it settles on
// English anyway, so listing one would be offering a choice that does nothing.
func languagePage() *shell.Page {
	return &shell.Page{
		Title: say.T("settings.language"),
		Build: func() ([]widget.Row, []func(int)) {
			now := say.Chosen()

			var (
				rows []widget.Row
				acts []func(int)
			)
			for _, tag := range say.Languages() {
				rows = append(rows, widget.Row{Label: say.Name(tag), Chosen: tag == now})
				acts = append(acts, useLanguage(tag))
			}
			return rows, acts
		},
	}
}

func useLanguage(tag string) func(int) {
	return func(int) { SetLanguage(tag) }
}
