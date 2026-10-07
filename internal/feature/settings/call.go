package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

func callPages() *sharedsettings.CallPages {
	return sharedsettings.NewCallPages(sharedsettings.CallOptions{Read: func() config.Call { return config.Get().Call }, Push: shell.Get().Push, SetStream: call.SetStream, SetIncoming: call.SetIncoming, SetAutoAnswer: call.SetAutoAnswer, SetAutoVideo: call.SetAutoVideo, SetPauseWake: call.SetPauseWake})
}
