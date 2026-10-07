package gui

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/widgets"
	"time"
)

func controls() style.Kit { return style.ByName(config.Get().Screen.Style).Kit }

var toolkit = sharedlib.Toolkit{Kit: controls, Pressable: pressable}
var panel = toolkit.Panel
var slider = toolkit.Slider
var seekBar = toolkit.SeekBar
var keyButton = toolkit.KeyButton
var wideKey = toolkit.WideKey
var keyed = toolkit.Keyed
var iconKey = toolkit.IconKey
var progressBar = toolkit.ProgressBar
var listRow = toolkit.ListRow
var tab = toolkit.Tab
var chosen = sharedlib.Chosen
var styleSample = sharedlib.StyleSample

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
