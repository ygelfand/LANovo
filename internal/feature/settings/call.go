package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

func callPages() *sharedsettings.CallPages {
	return sharedsettings.NewCallPages(
		sharedsettings.CallOptions{
			Read:          func() config.Call { return config.Get().Call },
			Push:          shell.Get().Push,
			SetStream:     call.Get().SetStream,
			SetIncoming:   call.Get().SetIncoming,
			SetAutoAnswer: call.Get().SetAutoAnswer,
			SetAutoVideo:  call.Get().SetAutoVideo,
			SetPauseWake:  call.Get().SetPauseWake,
		},
	)
}
