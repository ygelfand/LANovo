package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
)

const priorityUpgrade = widgets.UpgradePriority
const upgradeFrame = widgets.UpgradeFrame

func upgradeCard(w *gogui.Window) gogui.View {
	up := firmware.Get().Upgrade()
	if !up.Active() {
		return nil
	}
	pal := palette()
	img, key := ui.Logo(), "logo/light"
	if theme.Dark(pal.Background) {
		img, key = ui.Night(), "logo/night"
	}
	return widgets.UpgradeCard(w, widgets.Upgrade{Active: up.Active(), At: up.At, Version: up.Version}, pal, imageSrc(key, img))
}
