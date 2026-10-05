package gui

import (
	"strings"
	"sync"
	"unicode"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/style"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

var fields sync.Map

func textField(cfg gogui.InputCfg) gogui.View {
	fields.Store(cfg.ID, true)
	controls().Field(&cfg)
	return gogui.Input(cfg)
}

func typing(w *gogui.Window) bool {
	_, ok := fields.Load(w.FocusID())
	return ok
}

type keys struct {
	shift   bool
	symbols bool
}

var kb keys

var (
	letterRows = []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"}
	symbolRows = []string{"1234567890", "-/:;()&@\"", ".,?!'#"}
)

func (a *App) keyboard(w *gogui.Window) gogui.View {
	a.mu.Lock()
	r := a.r
	a.mu.Unlock()
	if r == nil {
		return nil
	}
	pal := palette()
	vw, vh := w.WindowSize()
	size := config.Get().Screen.Keyboard
	rowScale, textScale := keySize(size)
	gap := reach() * 0.12
	pad := reach() * 0.2
	high := reach() * rowScale
	unit := min((float32(vw)-2*pad-9*gap)/10, high*1.6)

	plain := pal.Background.Blend(pal.Text, 0.18)
	special := pal.Background.Blend(pal.Text, 0.10)
	key := func(id, label string, width float32, fill theme.Color, do func()) gogui.View {
		state := style.Rest
		if fill == special {
			state = style.Partial
		}
		st := gogui.CurrentTheme().Cfg.TextStyleDef
		if label == gogui.IconArrowUp || label == gogui.IconArrowLeft {
			st = gogui.CurrentTheme().TextStyleIconMedium
		}
		st.Size *= textScale
		st.Color = color(pal.Text)
		cfg := gogui.ContainerCfg{
			ID:      "key-" + id,
			Width:   width,
			Height:  high,
			Sizing:  gogui.FixedFixed,
			Radius:  gogui.RadiusMedium,
			Color:   color(fill),
			HAlign:  gogui.HAlignCenter,
			VAlign:  gogui.VAlignMiddle,
			Padding: gogui.NoPadding,
			OnClick: func(e gogui.EventCtx) {
				do()
				e.Window.InvalidateLayout()
				e.Consume()
			},
		}
		controls().Key(&cfg, &st, state)
		cfg.Content = []gogui.View{gogui.Label(label, st)}
		return gogui.Row(cfg)
	}

	typeRune := func(ch rune) func() {
		return func() {
			if kb.shift {
				ch = unicode.ToUpper(ch)
				kb.shift = false
			}
			r.Type(ch)
		}
	}

	rows := letterRows
	if kb.symbols {
		rows = symbolRows
	}
	var lines []gogui.View
	for i, row := range rows {
		var line []gogui.View
		if i == 2 {
			label := gogui.IconArrowUp
			if kb.symbols {
				label = ""
			}
			line = append(line, key("shift", label, unit*1.5+gap*0.5, special, func() {
				if !kb.symbols {
					kb.shift = !kb.shift
				}
			}))
		}
		for _, ch := range row {
			label := string(ch)
			if kb.shift && !kb.symbols {
				label = strings.ToUpper(label)
			}
			line = append(line, key(string(ch), label, unit, plain, typeRune(ch)))
		}
		if i == 2 {
			line = append(line, key("back", gogui.IconArrowLeft, unit*1.5+gap*0.5, special, func() { r.Press(gogui.KeyBackspace) }))
		}
		lines = append(lines, keyRow(line, gap))
	}

	mode := "123"
	if kb.symbols {
		mode = "abc"
	}
	lines = append(lines, keyRow([]gogui.View{
		key("mode", mode, unit*2+gap, special, func() { kb.symbols, kb.shift = !kb.symbols, false }),
		key("space", " ", unit*5+gap*4, plain, func() { r.Type(' ') }),
		key("done", say.T("keyboard.done"), unit*3+gap*2, special, func() { w.ClearFocus() }),
	}, gap))

	tall := int(4*high + 3*gap + pad*2.5)
	at := ui.Rect{Y: vh - tall, W: vw, H: tall}
	var corners gogui.Radius
	if size == config.KeyboardCompact {
		at.W = min(int(10*unit+9*gap+2*pad)+1, vw)
		at.X = vw - at.W
		corners = gogui.RadiusLarge
	}
	tray := gogui.ContainerCfg{
		ID:      "keyboard",
		Sizing:  gogui.FillFill,
		Padding: gogui.NewPadding(pad, pad, pad*1.5, pad),
		Spacing: gogui.SpacingPx(gap),
		Color:   color(pal.Background.Blend(pal.Text, 0.04)),
		Radius:  corners,
		HAlign:  gogui.HAlignCenter,
		Content: lines,
		OnClick: func(e gogui.EventCtx) { e.Consume() },
	}
	controls().Panel(&tray)
	return placed(at, gogui.Column(tray))
}

func keySize(k config.KeyboardSize) (row, text float32) {
	switch k {
	case config.KeyboardStandard:
		return 1, 1.15
	case config.KeyboardLarge:
		return 1.2, 1.25
	}
	return 0.75, 1
}

func keyRow(keys []gogui.View, gap float32) gogui.View {
	return gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		Padding: gogui.NoPadding,
		Spacing: gogui.SpacingPx(gap),
		HAlign:  gogui.HAlignCenter,
		Content: keys,
	})
}
