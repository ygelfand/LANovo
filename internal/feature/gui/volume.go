package gui

import (
	"fmt"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
)

func volumeScreen(v shell.View) *Screen {
	return &Screen{
		View:  v,
		Build: volumeCard,
	}
}

func volumeCard(w *gogui.Window) gogui.View {
	t := gogui.CurrentTheme()
	vw, vh := w.WindowSize()
	vol := volume.Get()
	stream, open := vol.Picked()
	level := vol.Level(stream)

	capsule := []gogui.View{
		gogui.Label(fmt.Sprintf("%d", level), t.TextStyleTitle),
		grip(gogui.Slider(gogui.SliderCfg{
			ID:        "volume-card",
			Vertical:  true,
			Value:     float32(100 - level),
			Color:     t.Cfg.ColorAccent,
			ColorLeft: t.Cfg.ColorInterior,
			Max:       100,
			Width:     reach(),
			Size:      reach() * 0.3,
			ThumbSize: reach() * 0.65,
			Height:    float32(vh) * 0.45,
			OnChange: func(v float32, e gogui.EventCtx) {
				vol.Set(stream, 100-int(v+0.5))
				vol.Linger()
				e.Window.InvalidateLayout()
			},
		})),
		pressable(gogui.Row, gogui.ContainerCfg{
			ID:      "volume-stream",
			Padding: gogui.PaddingSmall,
			Radius:  gogui.RadiusMedium,
			Content: []gogui.View{gogui.Label(stream.Label(), secondary())},
		}, func(gogui.EventCtx) { vol.Expand() }),
	}
	if open {
		for _, s := range config.Streams() {
			if s == stream {
				continue
			}
			pick := s
			capsule = append(capsule, pressable(gogui.Row, gogui.ContainerCfg{
				ID:      "volume-pick-" + string(s),
				Padding: gogui.PaddingSmall,
				Radius:  gogui.RadiusMedium,
				Content: []gogui.View{gogui.Label(s.Label(), gogui.TextStyle{})},
			}, func(gogui.EventCtx) { vol.Pick(pick) }))
		}
	}

	card := gogui.Column(gogui.ContainerCfg{
		ID:      "volume",
		Color:   t.Cfg.ColorPanel,
		Radius:  gogui.RadiusLarge,
		Padding: gogui.PaddingMedium,
		Spacing: gogui.SpacingMedium,
		HAlign:  gogui.HAlignCenter,
		OnClick: func(e gogui.EventCtx) {
			vol.Linger()
			e.Consume()
		},
		Content: capsule,
	})

	side := gogui.HAlignRight
	if config.Get().Screen.Volume == config.EdgeLeft {
		side = gogui.HAlignLeft
	}
	return pressable(gogui.Column, gogui.ContainerCfg{
		ID:      "volume-away",
		Width:   float32(vw),
		Height:  float32(vh),
		Sizing:  gogui.FixedFixed,
		Padding: gogui.PaddingLarge,
		HAlign:  side,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{card},
	}, func(gogui.EventCtx) { vol.Dismiss() })
}
