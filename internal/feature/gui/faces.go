package gui

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"strings"

	"github.com/go-gui-org/go-glyph"
	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type faceView func(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View

var faces = map[config.Face]faceView{
	config.FacePlain:         plainFace,
	config.FaceStack:         stackFace,
	config.FaceCards:         cardsFace,
	config.FaceAnalog:        analogFace(false),
	config.FaceAnalogSeconds: analogFace(true),
	config.FaceWords:         wordsFace,
	config.FaceOverlap:       overlapFace,
	config.FaceSegments:      segmentsFace,
}

func lettering(weight glyph.Typeface, size float32, c theme.Color) gogui.TextStyle {
	st := gogui.CurrentTheme().Cfg.TextStyleDef
	st.Typeface, st.Size, st.Color = weight, max(size, 1), color(c)
	return st
}

func fitted(w *gogui.Window, text string, st gogui.TextStyle, maxW, maxH float32) float32 {
	size := max(maxH, 1)
	for range 8 {
		st.Size = size
		wide := w.TextWidth(text, st)
		if wide <= maxW || size <= 1 {
			return size
		}
		size = max(min(size*maxW/wide, size-1), 1)
	}
	return size
}

func split(r face.Reading) (hour, minute string, ok bool) {
	hour, minute, ok = strings.Cut(r.Time, ":")
	if ok && len(hour) == 1 {
		hour = "0" + hour
	}
	return hour, minute, ok
}

func centred(content ...gogui.View) gogui.ContainerCfg {
	return gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: content,
	}
}

func dateLine(r face.Reading, size float32, ink theme.Theme) []gogui.View {
	if !r.Dated() {
		return nil
	}
	return []gogui.View{gogui.Label(r.Date, lettering(glyph.TypefaceRegular, size, ink.Muted))}
}

func plainFace(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View {
	big := lettering(glyph.TypefaceBold, 0, ink.Text)
	size := fitted(w, r.Time+r.Suffix, big, float32(box.W)*0.86, float32(box.H)*0.34)
	big.Size = size

	line := []gogui.View{gogui.Label(r.Time, big)}
	if r.Suffix != "" {
		line = append(line, gogui.Label(r.Suffix, lettering(glyph.TypefaceBold, size*0.24, ink.Muted)))
	}
	cfg := centred(append([]gogui.View{
		gogui.Row(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(size * 0.08), Content: line}),
	}, dateLine(r, size*0.20, ink)...)...)
	cfg.Spacing = gogui.SpacingPx(size * 0.1)
	return gogui.Column(cfg)
}

func stackFace(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View {
	hour, minute, ok := split(r)
	if !ok {
		return plainFace(w, box, r, ink)
	}
	big := lettering(glyph.TypefaceBold, 0, ink.Text)
	size := fitted(w, hour, big, float32(box.W)*0.80, float32(box.H)*0.34)
	big.Size = size
	tight := gogui.SpacingPx(-size * 0.22)

	top := []gogui.View{gogui.Label(hour, big)}
	if r.Suffix != "" {
		top = append(top, gogui.Label(r.Suffix, lettering(glyph.TypefaceBold, size*0.22, ink.Muted)))
	}
	digits := gogui.Column(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: tight, Content: []gogui.View{
		gogui.Row(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(size * 0.06), Content: top}),
		gogui.Label(minute, big),
	}})
	block := append([]gogui.View{digits}, dateLine(r, size*0.13, ink)...)
	return gogui.Column(centred(gogui.Column(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(size * 0.08), Content: block})))
}

func cardsFace(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View {
	hour, minute, ok := split(r)
	if !ok {
		return plainFace(w, box, r, ink)
	}
	across := box.W >= box.H
	long, short := float32(box.W)*0.88, float32(box.H)*0.74
	if !across {
		long, short = float32(box.H)*0.74, float32(box.W)*0.88
	}
	side := min(short, long/2.055)
	gap := side * 0.055

	card := func(digits string, corner string) gogui.View {
		st := lettering(glyph.TypefaceBold, 0, ink.Text)
		st.Size = fitted(w, "00", st, side*0.74, side*0.54)
		content := []gogui.View{gogui.Label(digits, st)}
		if corner != "" {
			content = append(content, gogui.Column(gogui.ContainerCfg{
				Float: true, FloatAnchor: gogui.FloatTopRight, FloatTieOff: gogui.FloatTopRight,
				FloatOffsetX: -side * 0.09, FloatOffsetY: side * 0.09, Padding: gogui.NoPadding,
				Content: []gogui.View{gogui.Label(corner, lettering(glyph.TypefaceBold, side*0.15, ink.Muted))},
			}))
		}
		return gogui.Column(gogui.ContainerCfg{
			Width: side, Height: side, Sizing: gogui.FixedFixed,
			HAlign: gogui.HAlignCenter, VAlign: gogui.VAlignMiddle,
			Color: color(ink.Surface), Radius: gogui.RadiusPx(side * 0.11), Padding: gogui.NoPadding,
			Content: content,
		})
	}
	pair := []gogui.View{card(hour, r.Suffix), card(minute, "")}
	var block gogui.View
	if across {
		block = gogui.Row(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(gap), Content: pair})
	} else {
		block = gogui.Column(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(gap), Content: pair})
	}
	cfg := centred(append([]gogui.View{block}, dateLine(r, side*0.085, ink)...)...)
	cfg.Spacing = gogui.SpacingPx(side * 0.16)
	return gogui.Column(cfg)
}

func analogFace(seconds bool) faceView {
	return func(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View {
		hour, minute, ok := clockHands(r)
		if !ok {
			return plainFace(w, box, r, ink)
		}
		bw, bh := float32(box.W), float32(box.H)
		dial := min(bw, bh) * 0.78
		version := uint64(hour*3600+minute*60) << 8
		if seconds {
			version += uint64(r.Second)
		}
		return gogui.DrawCanvas(gogui.DrawCanvasCfg{
			ID:      "analog",
			Width:   bw,
			Height:  bh,
			Version: version ^ uint64(box.W)<<40 ^ uint64(box.H)<<52,
			OnDraw: func(dc *gogui.DrawContext) {
				dateSize := dial * 0.075
				dateH, under := float32(0), float32(0)
				date := lettering(glyph.TypefaceRegular, dateSize, ink.Muted)
				if r.Dated() {
					dateH, under = dc.FontHeight(date), dial*0.10
				}
				cx := bw / 2
				cy := (bh-(dial+under+dateH))/2 + dial/2

				num := lettering(glyph.TypefaceBold, dial*0.25, ink.Text)
				nh := dc.FontHeight(num)
				for i, text := range []string{"12", "3", "6", "9"} {
					dx, dy := heading(float64(i) * 90)
					x := cx + dial*0.36*dx
					y := cy + dial*0.36*dy
					dc.Text(x-dc.TextWidth(text, num)/2, y-nh/2, text, num)
				}

				width := dial * 0.058
				dc.FilledPolygon(hand(cx, cy, dial*0.28, width, float64(hour%12)*30+float64(minute)*0.5), color(ink.Text))
				dc.FilledPolygon(hand(cx, cy, dial*0.44, width*0.70, float64(minute)*6), color(ink.Text))
				if seconds {
					dc.FilledPolygon(hand(cx, cy, dial*0.47, width*0.28, float64(r.Second)*6), color(ink.Accent))
				}
				dc.FilledCircle(cx, cy, dial*0.0375, color(ink.Text))

				if r.Dated() {
					dc.Text(cx-dc.TextWidth(r.Date, date)/2, cy+dial/2+under, r.Date, date)
				}
			},
		})
	}
}

func clockHands(r face.Reading) (hour, minute int, ok bool) {
	if _, err := fmt.Sscanf(r.Time, "%d:%d", &hour, &minute); err != nil {
		return 0, 0, false
	}
	return hour, minute, true
}

func heading(deg float64) (dx, dy float32) {
	rad := deg * math.Pi / 180
	return float32(math.Sin(rad)), float32(-math.Cos(rad))
}

func hand(cx, cy, length, width float32, deg float64) []float32 {
	dx, dy := heading(deg)
	px, py := -dy, dx
	tipX, tipY := cx+dx*length, cy+dy*length
	tail := length * 0.16
	tailX, tailY := cx-dx*tail, cy-dy*tail
	half := width / 2
	tip := half * 0.55
	return []float32{
		tailX + px*half, tailY + py*half,
		tipX + px*tip, tipY + py*tip,
		tipX - px*tip, tipY - py*tip,
		tailX - px*half, tailY - py*half,
	}
}

func imageSrc(key string, img image.Image) string {
	if img == nil {
		return ""
	}
	if gogui.HasImage(key) {
		return "mem:" + key
	}
	b := img.Bounds()
	n, ok := img.(*image.NRGBA)
	if !ok || n.Rect.Min != (image.Point{}) || n.Stride != b.Dx()*4 {
		n = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	}
	return gogui.UseImage(key, b.Dx(), b.Dy(), n.Pix)
}

func wordsFace(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View {
	hour, minute, ok := clockHands(r)
	if !ok {
		return plainFace(w, box, r, ink)
	}
	said := face.Say(hour, minute)
	big := lettering(glyph.TypefaceBold, 0, ink.Text)
	longest := ""
	for _, line := range said {
		if len(line) > len(longest) {
			longest = line
		}
	}
	size := fitted(w, longest, big, float32(box.W)*0.86, float32(box.H)*0.26)
	big.Size = size

	var lines []gogui.View
	for _, line := range said {
		lines = append(lines, gogui.Label(line, big))
	}
	block := gogui.Column(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(-size * 0.1), Content: lines})
	cfg := centred(append([]gogui.View{block}, dateLine(r, size*0.26, ink)...)...)
	cfg.Spacing = gogui.SpacingPx(size * 0.3)
	return gogui.Column(cfg)
}

func overlapFace(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View {
	hour, minute, ok := split(r)
	if !ok {
		return plainFace(w, box, r, ink)
	}
	big := lettering(glyph.TypefaceBold, 0, ink.Text)
	size := fitted(w, hour, big, float32(box.W)*0.78/1.18, float32(box.H)*0.46)
	big.Size = size
	faint := big
	faint.Color = color(ink.Muted)

	top := []gogui.View{gogui.Label(hour, faint)}
	if r.Suffix != "" {
		top = append(top, gogui.Label(r.Suffix, lettering(glyph.TypefaceBold, size*0.20, ink.Text)))
	}
	pair := gogui.Column(gogui.ContainerCfg{Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(-size * 0.45), Content: []gogui.View{
		gogui.Row(gogui.ContainerCfg{Padding: gogui.NoPadding, Content: top}),
		gogui.Row(gogui.ContainerCfg{Padding: gogui.NewPadding(0, 0, 0, size*0.18), Content: []gogui.View{gogui.Label(minute, big)}}),
	}})
	cfg := centred(append([]gogui.View{pair}, dateLine(r, size*0.13, ink)...)...)
	cfg.Spacing = gogui.SpacingPx(size * 0.1)
	return gogui.Column(cfg)
}

func segmentsFace(w *gogui.Window, box ui.Rect, r face.Reading, ink theme.Theme) gogui.View {
	hour, minute, ok := split(r)
	if !ok {
		return plainFace(w, box, r, ink)
	}
	rows := []string{hour + ":" + minute}
	if box.H > box.W {
		rows = []string{hour, minute}
	}
	unit := func(row string) float32 {
		var wide float32
		for i, c := range row {
			if i > 0 {
				wide += 0.16
			}
			if c == ':' {
				wide += 0.30
			} else {
				wide += 0.56
			}
		}
		return wide
	}
	widest := float32(0)
	for _, row := range rows {
		widest = max(widest, unit(row))
	}
	bw, bh := float32(box.W), float32(box.H)
	height := min(bw*0.88/widest, bh*0.34*1.5/float32(len(rows)))
	gap := height * 0.22
	tall := height*float32(len(rows)) + gap*float32(len(rows)-1)

	on := color(ink.Text)
	off := color(ink.Text.Blend(ink.Background, 0.91))
	digits := gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: "segments", Width: bw, Height: tall,
		Version: uint64(len(r.Time))<<48 ^ uint64(box.W)<<32 ^ uint64(box.H)<<16 ^ uint64(r.Time[len(r.Time)-1]),
		OnDraw: func(dc *gogui.DrawContext) {
			y := float32(0)
			for _, row := range rows {
				x := (bw - unit(row)*height) / 2
				for i, c := range row {
					if i > 0 {
						x += height * 0.16
					}
					if c == ':' {
						dot := max(height*0.15, 2)
						span := height * 0.30
						for _, cy := range []float32{y + height/3, y + height*2/3} {
							dc.FilledCircle(x+span/2, cy, dot/2, on)
						}
						x += span
						continue
					}
					cell := ui.Rect{X: int(x), Y: int(y), W: int(height * 0.56), H: int(height)}
					lit := face.Lit(c)
					for k, bar := range face.Bars(cell) {
						paint := off
						if lit[k] {
							paint = on
						}
						dc.FilledRoundedRect(float32(bar.X), float32(bar.Y), float32(bar.W), float32(bar.H), float32(min(bar.W, bar.H))/2, paint)
					}
					x += height * 0.56
				}
				y += height + gap
			}
		},
	})
	cfg := centred(append([]gogui.View{digits}, dateLine(r, height*0.13, ink)...)...)
	cfg.Spacing = gogui.SpacingPx(height * 0.34)
	return gogui.Column(cfg)
}
