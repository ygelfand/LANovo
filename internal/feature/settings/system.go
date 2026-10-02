package settings

import (
	"log/slog"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

func systemPage() *shell.Page {
	return &shell.Page{
		Title: say.T("system.title"),
		Build: func() ([]widget.Row, []func(int)) {
			return []widget.Row{
					{Glyph: gogui.IconTextSize, Label: say.T("settings.language"), Kind: widget.Chevron, Value: say.Name(say.Chosen())},
					{Glyph: gogui.IconClock, Label: say.T("datetime.title"), Kind: widget.Chevron, Value: zoneLabel()},
					{Glyph: gogui.IconKeyboard, Label: say.T("keyboard.size"), Kind: widget.Chevron, Value: config.Get().Screen.Keyboard.Label()},
					{Glyph: gogui.IconSync, Label: say.T("settings.power"), Kind: widget.Chevron},
				}, []func(int){
					open(languagePage()),
					open(dateTimePage()),
					open(keyboardPage()),
					open(powerPage()),
				}
		},
	}
}

func keyboardPage() *shell.Page {
	return &shell.Page{
		Title: say.T("keyboard.size"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Screen.Keyboard
			var rows []widget.Row
			var taps []func(int)
			for _, k := range config.KeyboardSizes() {
				rows = append(rows, widget.Row{Label: k.Label(), Chosen: k == now})
				taps = append(taps, func(int) { SetKeyboard(k) })
			}
			return rows, taps
		},
	}
}

func SetKeyboard(k config.KeyboardSize) {
	if err := config.Set().Screen().Keyboard(k); err != nil {
		slog.Error("the keyboard size could not be saved", "err", err)
	}
	shell.Get().Redraw()
}

func dateTimePage() *shell.Page {
	return &shell.Page{
		Title: say.T("datetime.title"),
		Build: func() ([]widget.Row, []func(int)) {
			cfg := config.Get()
			return []widget.Row{
					{Label: say.T("clock.zone"), Kind: widget.Chevron, Value: zoneLabel()},
					{Label: say.T("clock.hours"), Kind: widget.Toggle, On: cfg.Screen.Hours == config.TwentyFourHour},
				}, []func(int){
					open(zonePage()),
					func(int) { toggleHours(cfg.Screen.Hours) },
				}
		},
	}
}
