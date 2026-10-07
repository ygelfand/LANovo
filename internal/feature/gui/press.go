package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/interaction"
)

var interactions = sharedlib.New()

func pressable(layout func(gogui.ContainerCfg) gogui.View, cfg gogui.ContainerCfg, do func(gogui.EventCtx)) gogui.View {
	return interactions.Pressable(layout, cfg, do)
}
func release(e gogui.EventCtx) { interactions.Release(e) }
func lift(w *gogui.Window)     { interactions.Lift(w) }

var tapped = sharedlib.Tapped
var disc = sharedlib.Disc

func button(id, glyph string, st gogui.TextStyle, fill gogui.Color, do func(gogui.EventCtx)) gogui.View {
	return interactions.Button(id, glyph, st, fill, do)
}
