package settings

import (
	"log/slog"

	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/ui/widget"
	"github.com/ygelfand/LANovo/internal/update"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

// powerPage is the two ways to make the device go away and come back.
//
// On the panel as well as in Home Assistant, because the thing most likely to need restarting is
// what talks to Home Assistant.
func powerPage() *shell.Page {
	return &shell.Page{
		Title: say.T("power.title"),
		Build: func() ([]widget.Row, []func(int)) {
			return []widget.Row{
					{Label: say.T("power.restart"), Kind: widget.Chevron},
					{Label: say.T("power.reboot"), Kind: widget.Chevron},
				}, []func(int){
					open(confirmPage(say.T("power.restart"),
						say.T("power.restart.what"), restartService)),
					open(confirmPage(say.T("power.reboot"),
						say.T("power.reboot.what"), rebootDevice)),
				}
		},
	}
}

// confirmPage asks once more before something that takes the device away.
//
// A second screen rather than a row that acts: this list is scrolled past with a thumb, and neither
// of these is worth doing by accident. The header is the way out, as on every other page.
func confirmPage(title, what string, do func()) *shell.Page {
	return &shell.Page{
		Title: title,
		Build: func() ([]widget.Row, []func(int)) {
			return []widget.Row{
					{Label: what, Kind: widget.Plain},
					{Label: title},
				}, []func(int){
					nil,
					func(int) { do() },
				}
		},
	}
}

func restartService() { update.Restart("asked for on the panel") }

func rebootDevice() {
	safe.Go("reboot", func() {
		slog.Warn("rebooting, asked for on the panel")
		if err := prop.Reboot(prop.Local); err != nil {
			slog.Error("rebooting failed", "err", err)
		}
	})
}
