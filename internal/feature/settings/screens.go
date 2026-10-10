package settings

import (
	gogui "github.com/go-gui-org/go-gui/gui"

	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	sharedvisual "github.com/ygelfand/libcountertop/pkg/display/visual"
	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/a2dp"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/dhcp"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/network"
	"github.com/ygelfand/LANovo/internal/feature/sendspin"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func root() *sharedshell.Page {
	return sharedsettings.RootPage(sharedsettings.RootDependencies{
		Network:      wifi.Get(),
		Screen:       config.ScreenSection,
		Basic:        basicPages(),
		Assistants:   assistantPages(),
		Home:         homecontrol.Get(),
		Shell:        shell.Get(),
		Version:      layout.Version,
		Volume:       sharedvolume.For(volume.Get(), config.StreamMedia),
		NetworkPage:  func() sharedshell.View { return networkPage() },
		FeaturesPage: func() sharedshell.View { return featuresPage() },
		VolumePage:   func() sharedshell.View { return volume.Page() },
		AboutPage:    func() sharedshell.View { return aboutPage() },
		Sections: func() []sharedsettings.Section {
			return []sharedsettings.Section{
				{
					Label: "settings.camera",
					Glyph: gogui.IconCamera,
					Page:  func() sharedshell.View { return cameraPage() },
				},
			}
		},
	})
}

func sensorsPage() *sharedshell.Page {
	return sharedsettings.PresencePage(
		sensors.Table,
		func() config.Presence { return config.Get().Presence },
		sensors.SetPresence,
	)
}

func visualPage() *sharedshell.Page {
	return visualPicker(say.T("debug.visuals"), "", nil, nil, func(k sharedvisual.Kind) {
		visuals.Get().SetKind(k)
		shell.Get().Push(visuals.Get().View())
	})
}

func visualPicker(
	title, empty string,
	none func(),
	chosen func() string,
	pick func(sharedvisual.Kind),
) *sharedshell.Page {
	return sharedsettings.VisualPicker(
		title,
		empty,
		none,
		chosen,
		pick,
		visual.Thumbs().Fit,
		previewKit(),
	)
}

func uiSize() string {
	if s := config.Get().Screen.Size; s != "" {
		return s
	}
	return board.Current().UISize
}

func networkPage() *sharedshell.Page {
	return sharedsettings.NetworkPage(func() sharedsettings.NetworkState {
		c := sharedsettings.NetworkState{
			SSID:    wifi.Get().Network(),
			Address: address(),
			MAC:     wifi.Get().MAC(),
			Verify:  config.Get().Network.Verify,
		}
		if l := dhcp.Get().Lease(); l != nil {
			c.Router = l.Router.String()
			c.Renew = l.Renew
		}
		return c
	}, network.Get().SetVerify)
}

func callsPage() *sharedshell.Page { return callPages().Calls() }

func castPage() *sharedshell.Page { return servicePages().Cast() }

func featuresPage() *sharedshell.Page {
	return sharedsettings.FeaturesPage(func() []sharedsettings.Feature {
		c := config.Get()
		return []sharedsettings.Feature{
			{Key: "features.sendspin", On: c.Sendspin.Enabled, Set: sendspin.Get().SetEnabled},
			{Key: "features.cast", Page: func() sharedshell.View { return castPage() }},
			{Key: "call.settings", Page: func() sharedshell.View { return callsPage() }},
			{Key: "features.proxy", On: c.Bluetooth.Proxy, Set: bluetooth.Get().SetProxy},
			{Key: "features.speaker", On: c.Bluetooth.Speaker, Set: a2dp.Get().SetEnabled},
		}
	}, shell.Get().Push)
}

func aboutPage() *sharedshell.Page {
	return sharedsettings.AboutPage(func() sharedsettings.About {
		return sharedsettings.About{
			Name:    config.Get().Device.Name,
			Version: layout.Version,
			Built:   layout.BuildDate,
			Commit:  layout.GitCommit,
		}
	})
}

func address() string {
	l := dhcp.Get().Lease()
	if l == nil {
		return say.T("network.address.none")
	}
	return l.Address.IP.String()
}
