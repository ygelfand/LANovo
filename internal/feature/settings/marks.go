package settings

import (
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	tesseraview "github.com/ygelfand/libcountertop/pkg/tessera/widgets"

	lanovoui "github.com/ygelfand/LANovo/internal/ui"
)

func modeMarks() sharedsettings.ModeMarks {
	return sharedsettings.ModeMarks{
		Tessera: tesseraview.Mark,
		Native:  sharedsettings.Themed(lanovoui.Logo().Light, lanovoui.Logo().Dark),
	}
}
