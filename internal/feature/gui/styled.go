package gui

import (
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/LANovo/internal/ui/style"
)

func controls() style.Kit { return style.ByName(config.Get().Screen.Style).Kit }

func panel(cfg gogui.ContainerCfg) gogui.ContainerCfg {
	cfg.Color, cfg.Radius = gogui.CurrentTheme().Cfg.ColorPanel, gogui.RadiusLarge
	controls().Panel(&cfg)
	return cfg
}

func slider(cfg gogui.SliderCfg) gogui.View {
	controls().Slider(&cfg)
	return gogui.Slider(cfg)
}

func seekBar(cfg gogui.SliderCfg, marks []style.Span) gogui.View {
	cfg.Look = controls().Seek(marks)
	return gogui.Slider(cfg)
}

func keyButton(id, glyph string, st gogui.TextStyle, state style.State, do func(gogui.EventCtx)) gogui.View {
	side := st.Size * 2
	return keyed(gogui.ContainerCfg{ID: id, Width: side, Height: side, Sizing: gogui.FixedFixed, Radius: gogui.RadiusLarge, Color: gogui.CurrentTheme().Cfg.ColorPanel}, glyph, st, state, do)
}

func wideKey(id, glyph string, st gogui.TextStyle, height float32, state style.State, do func(gogui.EventCtx)) gogui.View {
	return keyed(gogui.ContainerCfg{ID: id, Height: height, Sizing: gogui.FillFixed, Radius: gogui.RadiusMedium, Color: gogui.CurrentTheme().Cfg.ColorBackground}, glyph, st, state, do)
}

func keyed(cfg gogui.ContainerCfg, glyph string, st gogui.TextStyle, state style.State, do func(gogui.EventCtx)) gogui.View {
	t := gogui.CurrentTheme().Cfg
	cfg.Padding = gogui.NoPadding
	cfg.HAlign = gogui.HAlignCenter
	cfg.VAlign = gogui.VAlignMiddle
	st.Color = t.TextStyleDef.Color
	if state == style.Chosen {
		cfg.Color, st.Color = t.ColorAccent, t.ColorBackground
	}
	controls().Key(&cfg, &st, state)
	cfg.Content = []gogui.View{gogui.Label(glyph, st)}
	return pressable(gogui.Row, cfg, do)
}

func iconKey(cfg gogui.ContainerCfg, glyph string, st gogui.TextStyle, state style.State, do func(gogui.EventCtx)) gogui.View {
	controls().Key(&cfg, &st, state)
	cfg.Content = []gogui.View{gogui.Label(glyph, st)}
	return pressable(gogui.Row, cfg, do)
}

func progressBar(cfg gogui.ProgressBarCfg) gogui.View {
	controls().Progress(&cfg)
	return gogui.ProgressBar(cfg)
}

func listRow(cfg *gogui.ContainerCfg, state style.State) {
	switch state {
	case style.Chosen:
		highlight(cfg)
	case style.Partial:
		outline(cfg)
	}
	controls().Row(cfg, state)
}

func tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state style.State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.ColorPanel, gogui.RadiusLarge, t.TextStyleDef.Color
	if state == style.Chosen {
		cfg.Color, st.Color = t.ColorAccent, t.ColorBackground
	}
	controls().Tab(cfg, st, state)
}

func chosen(on bool) style.State {
	if on {
		return style.Chosen
	}
	return style.Rest
}

func styleSample(name string, w, h float32) gogui.View {
	st := style.ByName(name)
	t := gogui.CurrentTheme()
	face := t.Cfg.TextStyleDef
	face.Family, face.Size = st.Family, face.Size*1.3
	mark := t.TextStyleIconLarge
	mark.Size = reach() * 0.4
	key := gogui.ContainerCfg{Width: reach() * 0.9, Height: reach() * 0.9, Sizing: gogui.FixedFixed, Color: t.Cfg.ColorAccent, Radius: gogui.RadiusLarge, HAlign: gogui.HAlignCenter, VAlign: gogui.VAlignMiddle, Padding: gogui.NoPadding}
	mark.Color = t.Cfg.ColorBackground
	st.Kit.Key(&key, &mark, style.Chosen)
	key.Content = []gogui.View{gogui.Label(gogui.IconPlay, mark)}
	level := gogui.SliderCfg{ID: "sample-" + name, Value: 60, Max: 100, Sizing: gogui.FillFit, Height: reach() * 0.7}
	st.Kit.Slider(&level)
	card := gogui.ContainerCfg{
		Width:   w,
		Height:  h,
		Sizing:  gogui.FixedFixed,
		Color:   t.Cfg.ColorPanel,
		Radius:  gogui.RadiusLarge,
		Padding: gogui.PaddingMedium,
		Spacing: gogui.SpacingSmall,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{
			gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, VAlign: gogui.VAlignMiddle, Spacing: gogui.SpacingSmall, Content: []gogui.View{
				gogui.Label("Aa", face), gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding}), st.Kit.Toggle("sample-toggle-"+name, true),
			}}),
			gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, VAlign: gogui.VAlignMiddle, Spacing: gogui.SpacingSmall, Content: []gogui.View{
				gogui.Slider(level), gogui.Row(key),
			}}),
		},
	}
	st.Kit.Panel(&card)
	return gogui.Column(card)
}

func spans(marks []videoplayer.Mark, length time.Duration) []style.Span {
	if length <= 0 {
		return nil
	}
	out := make([]style.Span, 0, len(marks))
	for _, m := range marks {
		out = append(out, style.Span{From: float32(m.From) / float32(length), To: float32(m.To) / float32(length), Color: color(m.Color)})
	}
	return out
}
