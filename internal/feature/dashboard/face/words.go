package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(config.FaceWords, words{}) }

// words says the time the way somebody asked would say it.
//
// The slowest face to read, and that is the choice being offered rather than a fault in it: a clock
// that has to be read instead of glanced at is a different object in a room, and somebody who wants
// the minute exactly has five other faces.
//
// The English is in phrase.go. This file sets it.
type words struct{}

// The block, against the box.
const (
	wordsWidthShare  = 0.86
	wordsHeightShare = 0.62
	wordsLineOfBox   = 0.26
)

// Against the line, so the face keeps its proportions at any size.
const (
	wordsLineGap = 0.12
	wordsDateOf  = 0.26
	wordsDateGap = 0.55
)

func (words) Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme) {
	h, m, ok := clock(r)
	if !ok {
		plain{}.Draw(s, in, r, palette)
		return
	}

	said := Say(h, m)

	size := wordsFit(in, said)
	font := ui.MustLoad(ui.Bold, size)
	date := ui.MustLoad(ui.Regular, max(int(float64(size)*wordsDateOf), 1))

	_, lineH := font.Measure("x")
	gap := int(float64(lineH) * wordsLineGap)

	dateW, dateH, under := 0, 0, 0
	if r.Dated() {
		dateW, dateH = date.Measure(r.Date)
		under = int(float64(lineH) * wordsDateGap)
	}

	tall := lineH*len(said) + gap*(len(said)-1) + under + dateH
	top := in.Y + (in.H-tall)/2

	// Left aligned to the widest line rather than each line centered: the phrase is a sentence, and
	// a sentence that steps in and out at both ends reads as a poster instead.
	var widest int
	for _, line := range said {
		w, _ := font.Measure(line)
		widest = max(widest, w)
	}
	left := in.X + (in.W-widest)/2

	for i, line := range said {
		ui.DrawText(s, font, left, top+i*(lineH+gap), palette.Text, palette.Background, line)
	}

	if r.Dated() {
		ui.DrawText(s, date, max(in.X, min(left, in.X+in.W-dateW)),
			top+len(said)*lineH+(len(said)-1)*gap+under,
			palette.Muted, palette.Background, r.Date)
	}
}

// wordsFit is the largest size at which the longest line fits the width and they all fit the height.
func wordsFit(in ui.Rect, said []string) int {
	longest := ""
	for _, line := range said {
		if len(line) > len(longest) {
			longest = line
		}
	}

	size := fit(ui.Bold, longest,
		int(float64(in.W)*wordsWidthShare), int(float64(in.H)*wordsLineOfBox))

	for range 8 {
		font := ui.MustLoad(ui.Bold, size)

		_, lineH := font.Measure("x")
		tall := lineH*len(said) + int(float64(lineH)*wordsLineGap)*(len(said)-1)

		room := int(float64(in.H) * wordsHeightShare)
		if tall <= room || size <= 1 {
			return size
		}

		size = max(min(size*room/tall, size-1), 1)
	}
	return size
}
