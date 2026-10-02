package gui

import (
	"math"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

const (
	priorityUpgrade = 200
	upgradeFrame    = 33 * time.Millisecond
)

func upgradeCard(w *gogui.Window) gogui.View {
	up := firmware.Get().Upgrade()
	if !up.Active() {
		return nil
	}
	pal := palette()
	accent := gogui.CurrentTheme().Cfg.ColorAccent
	track := color(pal.Text).WithOpacity(0.12)
	vw, vh := w.WindowSize()
	side := float32(min(vw, vh)) * 0.42
	installing := up.At >= 1
	spin := float32(math.Mod(float64(time.Now().UnixMilli())/1100.0, 1)) * 2 * math.Pi

	ring := gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: "upgrade-ring", Width: side, Height: side,
		Version: uint64(time.Now().UnixNano()),
		OnDraw: func(dc *gogui.DrawContext) {
			c, stroke := side/2, side*0.11
			r := c - stroke/2
			top := float32(-math.Pi / 2)
			dc.Arc(c, c, r, r, 0, 2*math.Pi, track, stroke)
			if installing {
				dc.Arc(c, c, r, r, 0, 2*math.Pi, accent.WithOpacity(0.55), stroke)
				dc.Arc(c, c, r, r, spin, math.Pi*0.5, accent, stroke)
				return
			}
			if at := min(max(up.At, 0), 1); at > 0 {
				dc.Arc(c, c, r, r, top, at*2*math.Pi, accent, stroke)
			}
		},
	})

	mark := int(side * 0.42)
	img, name := ui.Logo(), "logo/light"
	if theme.Dark(pal.Background) {
		img, name = ui.Night(), "logo/night"
	}
	inset := (side - float32(mark)) / 2
	dial := gogui.Column(gogui.ContainerCfg{
		Width:   side,
		Height:  side,
		Sizing:  gogui.FixedFixed,
		Padding: gogui.NoPadding,
		Content: []gogui.View{
			ring,
			gogui.Column(gogui.ContainerCfg{
				Float:        true,
				FloatOffsetX: inset,
				FloatOffsetY: inset,
				Padding:      gogui.NoPadding,
				Content:      []gogui.View{picture(imageSrc(name, img), mark, mark)},
			}),
		},
	})

	status := say.F("upgrade.percent", map[string]any{"N": int(up.At*100 + 0.5)})
	if installing {
		status = say.T("upgrade.installing")
	}
	line := say.F("upgrade.line", map[string]any{"Version": up.Version, "Status": status})

	return gogui.Column(gogui.ContainerCfg{
		ID:      "upgrade",
		Sizing:  gogui.FillFill,
		Color:   color(pal.Background),
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Spacing: gogui.SpacingLarge,
		Content: []gogui.View{dial, gogui.Label(line, secondary())},
		OnClick: func(e gogui.EventCtx) { e.Consume() },
	})
}
