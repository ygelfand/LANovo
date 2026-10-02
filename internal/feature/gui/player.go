package gui

import (
	"fmt"
	"sync"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui"
)

const (
	seekSettle = 250 * time.Millisecond
	seekLand   = 3 * time.Second
)

type scrub struct {
	mu    sync.Mutex
	sk    media.Seeker
	to    time.Duration
	held  bool
	until time.Time
	timer *time.Timer
}

func (s *scrub) move(sk media.Seeker, to time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sk, s.to, s.held = sk, to, true
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(seekSettle, s.land)
}

func (s *scrub) land() {
	s.mu.Lock()
	sk, to := s.sk, s.to
	s.held, s.until = false, time.Now().Add(seekLand)
	s.mu.Unlock()
	if sk != nil && sk.CanSeek() {
		sk.Seek(to)
	}
}

func (s *scrub) shown(elapsed time.Duration) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held || time.Now().Before(s.until) {
		return s.to
	}
	return elapsed
}

var seeking scrub

func playerScreen(v shell.View) *Screen {
	return &Screen{
		Title: say.T("player.now"),
		View:  v,
		Build: playerBody,
	}
}

func still(img *ui.Image) string {
	w, h := img.Size()
	key := fmt.Sprintf("art/%p/%dx%d", img, w, h)
	if gogui.HasImage(key) {
		return "mem:" + key
	}
	return gogui.UseImage(key, w, h, nrgba(img))
}

func clockText(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func playerBody(w *gogui.Window) gogui.View {
	t := gogui.CurrentTheme()
	now := media.Get().Now()
	vw, vh := w.WindowSize()
	tall := vh > vw
	side := float32(vh) * 0.42
	if tall {
		side = float32(vw) * 0.6
	}

	var cover gogui.View
	if now.Art != nil {
		cover = gogui.Image(gogui.ImageCfg{Src: still(now.Art), Width: side, Height: side})
	} else {
		st := t.TextStyleIconXLarge
		st.Color = t.Cfg.ColorTextSecondary
		cover = gogui.Column(gogui.ContainerCfg{
			Width: side, Height: side, Sizing: gogui.FixedFixed,
			HAlign: gogui.HAlignCenter, VAlign: gogui.VAlignMiddle,
			Color: t.Cfg.ColorPanel, Radius: gogui.RadiusLarge,
			Content: []gogui.View{gogui.Label(gogui.IconMusic, st)},
		})
	}

	words := []gogui.View{gogui.Text(gogui.TextCfg{Text: media.Heading(now), TextStyle: t.TextStyleTitle, Mode: gogui.TextModeWrap})}
	if now.Artist != "" {
		words = append(words, gogui.Label(now.Artist, gogui.TextStyle{}))
	}
	if now.Album != "" {
		words = append(words, gogui.Label(now.Album, secondary()))
	}
	if from := media.Get().Named(); from != "" {
		words = append(words, gogui.Label(from, secondary()))
	}
	if now.Length > 0 {
		sk, _ := media.Transport().(media.Seeker)
		words = append(words, progress(now, sk, secondary()))
	}
	words = append(words, transport(now), loudness())

	text := gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Spacing: gogui.SpacingMedium, Content: words})
	main := gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Spacing: gogui.SpacingLarge, Content: []gogui.View{cover, text}})
	if tall {
		main = gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, HAlign: gogui.HAlignCenter, Spacing: gogui.SpacingLarge, Content: []gogui.View{cover, text}})
	}
	if len(now.Queue) == 0 {
		return main
	}
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		Spacing: gogui.SpacingLarge,
		Content: []gogui.View{main, queue(now)},
	})
}

func progress(now media.Now, sk media.Seeker, st gogui.TextStyle) gogui.View {
	at := seeking.shown(now.Elapsed)
	canSeek := sk != nil && sk.CanSeek()
	bar := grip(gogui.Slider(gogui.SliderCfg{
		ID:       "seek",
		Height:   reach(),
		Sizing:   gogui.FillFit,
		Value:    float32(at.Seconds()),
		Max:      float32(now.Length.Seconds()),
		Disabled: !canSeek,
		OnChange: func(v float32, e gogui.EventCtx) {
			seeking.move(sk, time.Duration(v*float32(time.Second)))
			e.Window.InvalidateLayout()
		},
	}))
	content := []gogui.View{bar}
	if now.LiveWithin == 0 {
		content = []gogui.View{gogui.Label(clockText(at), st), bar, gogui.Label(clockText(now.Length), st)}
	}
	return gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		VAlign:  gogui.VAlignMiddle,
		Spacing: gogui.SpacingMedium,
		Content: content,
	})
}

type control struct {
	glyph   string
	do      func(media.Source)
	need    media.Controls
	primary bool
}

func transport(now media.Now) gogui.View {
	middle := control{glyph: gogui.IconPlay, do: media.Source.Play, primary: true}
	if now.Playing && !now.Paused {
		middle = control{glyph: gogui.IconPause, do: media.Source.Pause, need: media.CanPause, primary: true}
	}
	t := gogui.CurrentTheme()
	var shown []gogui.View
	for i, b := range []control{
		{glyph: gogui.IconFastBackward, do: media.Source.Previous, need: media.CanPrevious},
		middle,
		{glyph: gogui.IconStop, do: media.Source.Stop, need: media.CanStop},
		{glyph: gogui.IconFastForward, do: media.Source.Next, need: media.CanNext},
	} {
		if b.need != 0 && !now.Can.Has(b.need) {
			continue
		}
		do := b.do
		st, fill := t.TextStyleIconLarge, t.Cfg.ColorPanel
		st.Color, st.Size = t.Cfg.TextStyleDef.Color, reach()*0.45
		if b.primary {
			fill = t.Cfg.ColorAccent
			st.Color, st.Size = t.Cfg.ColorBackground, reach()*0.75
		}
		shown = append(shown, button(fmt.Sprintf("transport-%d", i), b.glyph, st, fill, func(gogui.EventCtx) { do(media.Transport()) }))
	}
	return gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, VAlign: gogui.VAlignMiddle, Spacing: gogui.SpacingMedium, Content: shown})
}

func loudness() gogui.View {
	return gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		VAlign:  gogui.VAlignMiddle,
		Spacing: gogui.SpacingMedium,
		Content: []gogui.View{
			icon(gogui.IconSpeaker, gogui.CurrentTheme().Cfg.ColorTextSecondary),
			grip(gogui.Slider(gogui.SliderCfg{
				ID:     "media-volume",
				Height: reach(),
				Sizing: gogui.FillFit,
				Value:  float32(volume.Get().Level(config.StreamMedia)),
				Max:    100,
				OnChange: func(v float32, e gogui.EventCtx) {
					volume.Get().Set(config.StreamMedia, int(v+0.5))
					e.Window.InvalidateLayout()
				},
			})),
		},
	})
}

func queue(now media.Now) gogui.View {
	rows := []gogui.View{gogui.Label(say.T("player.next"), secondary())}
	for i, tr := range now.Queue {
		play := tr.Play
		line := []gogui.View{gogui.Label(tr.Title, gogui.TextStyle{})}
		if tr.Artist != "" {
			line = append(line, gogui.Label(tr.Artist, secondary()))
		}
		var onClick func(gogui.EventCtx)
		if play != nil {
			onClick = func(gogui.EventCtx) { play() }
		}
		content := []gogui.View{gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Content: line})}
		if tr.Length > 0 {
			content = append(content, gogui.Label(clockText(tr.Length), secondary()))
		}
		rows = append(rows, pressable(gogui.Row, gogui.ContainerCfg{
			ID:      fmt.Sprintf("queue-%d", i),
			Sizing:  gogui.FillFit,
			VAlign:  gogui.VAlignMiddle,
			Padding: gogui.PaddingSmall,
			Spacing: gogui.SpacingMedium,
			Radius:  gogui.RadiusMedium,
			Content: content,
		}, onClick))
	}
	return gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Spacing: gogui.SpacingSmall, Content: rows})
}

const priorityMini = 5

func (a *App) mini(w *gogui.Window) gogui.View {
	now := media.Get().Now()
	if shell.Get().Open() || !(now.Playing || now.Paused) {
		return nil
	}
	t := gogui.CurrentTheme()
	vw, vh := w.WindowSize()
	side := reach() * 1.2

	var art gogui.View
	if now.Art != nil {
		art = gogui.Image(gogui.ImageCfg{Src: still(now.Art), Width: side, Height: side})
	} else {
		st := t.TextStyleIconLarge
		st.Color = t.Cfg.ColorTextSecondary
		art = gogui.Column(gogui.ContainerCfg{Width: side, Height: side, Sizing: gogui.FixedFixed, HAlign: gogui.HAlignCenter, VAlign: gogui.VAlignMiddle, Padding: gogui.NoPadding, Content: []gogui.View{gogui.Label(gogui.IconMusic, st)}})
	}

	words := []gogui.View{gogui.Label(media.Heading(now), gogui.TextStyle{})}
	if now.Artist != "" {
		words = append(words, gogui.Label(now.Artist, secondary()))
	}

	mark := t.TextStyleIconLarge
	mark.Color = t.Cfg.TextStyleDef.Color
	mark.Size = reach() * 0.6
	type control struct {
		glyph string
		do    func(gogui.EventCtx)
	}
	middle := control{gogui.IconPlay, func(gogui.EventCtx) { media.Transport().Play() }}
	if now.Playing && !now.Paused {
		middle = control{gogui.IconPause, func(gogui.EventCtx) { media.Transport().Pause() }}
	}
	controls := []control{middle}
	if now.Can.Has(media.CanNext) {
		controls = append(controls, control{gogui.IconFastForward, func(gogui.EventCtx) { media.Transport().Next() }})
	}
	controls = append(controls, control{gogui.IconExpand, func(gogui.EventCtx) { media.Get().Open() }})

	content := []gogui.View{art, gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Content: words})}
	for i, c := range controls {
		content = append(content, pressable(gogui.Row, gogui.ContainerCfg{
			ID:      fmt.Sprintf("mini-%d", i),
			Width:   side,
			Height:  side,
			Sizing:  gogui.FixedFixed,
			HAlign:  gogui.HAlignCenter,
			VAlign:  gogui.VAlignMiddle,
			Padding: gogui.NoPadding,
			Radius:  gogui.RadiusMedium,
			Content: []gogui.View{gogui.Label(c.glyph, mark)},
		}, c.do))
	}
	bar := gogui.Row(gogui.ContainerCfg{
		ID:      "mini",
		Sizing:  gogui.FillFit,
		VAlign:  gogui.VAlignMiddle,
		Color:   t.Cfg.ColorPanel,
		Radius:  gogui.RadiusLarge,
		Padding: gogui.PaddingSmall,
		Spacing: gogui.SpacingMedium,
		Shadow:  &gogui.BoxShadow{Color: gogui.Black.WithOpacity(0.5), OffsetY: 4, BlurRadius: 18},
		OnClick: func(e gogui.EventCtx) { e.Consume() },
		Content: content,
	})
	return gogui.Column(gogui.ContainerCfg{
		Width:   float32(vw),
		Height:  float32(vh),
		Sizing:  gogui.FixedFixed,
		Padding: gogui.PaddingLarge,
		VAlign:  gogui.VAlignBottom,
		Content: []gogui.View{bar},
	})
}
