package boot

import (
	"log/slog"

	bootview "github.com/ygelfand/libcountertop/pkg/display/boot"
	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/layout"
)

func openBootScene(w, h int) *bootview.Scene {
	c := display.Get().Helper()
	if c == nil || c.Err() != nil || w <= 0 || h <= 0 {
		return nil
	}
	nw, nh := display.Get().Native()
	scene, err := bootview.NewScene(
		c,
		61,
		nw,
		nh,
		w,
		h,
		int(display.Get().Orientation()),
		chosen(),
		layout.Version,
		say.T("boot.skip"),
	)
	if err != nil {
		slog.Error("starting boot GUI", "err", err)
		return nil
	}
	return scene
}
