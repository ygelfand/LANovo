package gui

import (
	"embed"
	"fmt"
	"log/slog"

	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	sharedpeer "github.com/ygelfand/libcountertop/pkg/network/peer"
	"github.com/ygelfand/libcountertop/pkg/say"
)

//go:embed devices/*.svg
var devices embed.FS

const pictureAspect = 190.0 / 320.0

func devicePicture(codename string) (string, string) {
	if b, err := devices.ReadFile("devices/" + codename + ".svg"); err == nil {
		return "device-" + codename, string(b)
	}
	b, _ := devices.ReadFile("devices/default.svg")
	return "device-default", string(b)
}

func callsBoard(at ui.Rect, _ dashboard.Tab, pal theme.Theme) gogui.View {
	peers := discovery.Get().Peers()
	gap := reach() * 0.25
	cols := max(3, int(float32(at.W)/(reach()*4)))
	side := cell(float32(at.W)-gap, gap, cols)

	var views []gogui.View
	if len(peers) == 0 {
		views = append(views, gogui.Label(say.T("call.empty"), secondary()))
	}
	stacks := make([][]gogui.View, cols)
	for i, p := range peers {
		stacks[i%cols] = append(stacks[i%cols], peerTile(fmt.Sprintf("peer-%d", i), side, p, pal))
	}
	var columns []gogui.View
	for _, stack := range stacks {
		columns = append(
			columns,
			gogui.Column(
				gogui.ContainerCfg{
					Width:   side,
					Sizing:  gogui.FixedFit,
					Padding: gogui.NoPadding,
					Spacing: gogui.SpacingPx(gap),
					Content: stack,
				},
			),
		)
	}
	if len(peers) > 0 {
		views = append(
			views,
			gogui.Row(
				gogui.ContainerCfg{
					Sizing:  gogui.FillFit,
					Padding: gogui.NoPadding,
					Spacing: gogui.SpacingPx(gap),
					Content: columns,
				},
			),
		)
	}
	return placed(at, gogui.Column(gogui.ContainerCfg{
		ID:         "peers",
		Sizing:     gogui.FillFill,
		Padding:    gogui.NoPadding,
		Spacing:    gogui.SpacingPx(gap),
		Scrollable: true,
		OnGesture:  presentation.HoldStill,
		Content:    views,
	}))
}

func peerTile(id string, side float32, p sharedpeer.Peer, pal theme.Theme) gogui.View {
	t := gogui.CurrentTheme().Cfg
	inner := side - 2*t.PaddingMedium.Left
	key, svg := devicePicture(p.Board)

	glyph := iconStyle(t.TextStyleDef.Color)
	glyph.Size = reach() * 0.4
	action := func(suffix, icon string, do func(gogui.EventCtx)) gogui.View {
		return presentation.Toolkit.WideKey(id+"-"+suffix, icon, glyph, reach()*0.9, style.Partial, do)
	}
	var video func(gogui.EventCtx)
	if p.Video() && board.Current().CameraWidth > 0 {
		video = func(gogui.EventCtx) { dial(p, true) }
	}
	buttons := []gogui.View{
		action("audio", gogui.IconPhone, func(gogui.EventCtx) { dial(p, false) }),
		action("info", gogui.IconInfo, func(gogui.EventCtx) { call.Get().Open(p) }),
		action("video", gogui.IconVideo, video),
	}
	tall := min(reach()*1.4, inner*pictureAspect)

	cfg := gogui.ContainerCfg{
		ID:      id,
		Width:   side,
		Sizing:  gogui.FixedFit,
		Padding: gogui.PaddingMedium,
		Spacing: gogui.SpacingSmall,
		Radius:  gogui.RadiusLarge,
		Color:   color(pal.Surface),
		Content: []gogui.View{
			gogui.Label(p.Name, t.TextStyleDef),
			gogui.Row(
				gogui.ContainerCfg{
					Sizing:  gogui.FillFit,
					Padding: gogui.NoPadding,
					HAlign:  gogui.HAlignCenter,
					Content: []gogui.View{
						gogui.Svg(
							gogui.SvgCfg{
								ID:      id + "-" + key,
								SvgData: svg,
								Width:   tall / pictureAspect,
								Height:  tall,
							},
						),
					},
				},
			),
			gogui.Row(
				gogui.ContainerCfg{
					Sizing:  gogui.FillFit,
					Padding: gogui.NoPadding,
					Spacing: gogui.SpacingSmall,
					Content: buttons,
				},
			),
		},
	}
	presentation.Kit().Tile(&cfg, style.Rest)
	return gogui.Column(cfg)
}

func dial(p sharedpeer.Peer, video bool) {
	if err := call.Get().Start(p, video); err != nil {
		slog.Warn("call not placed", "to", p.Name, "err", err)
	}
}
