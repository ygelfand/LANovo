package configsync

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/syncview"
	shared "github.com/ygelfand/libcountertop/pkg/settings/configsync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/web"
)

var Get = sync.OnceValue(func() *shared.Sync {
	return shared.New(shared.Dependencies{
		Store: &component.Settings,
		Peers: discovery.Get(),
		Port:  web.Port,
		Stage: syncview.NewStage(shell.Get()),
	})
})

func init() {
	web.Handle("POST /config/offer", Get().OfferHandler())
	web.Handle("POST /config/ask", Get().AskHandler())
	web.Handle("POST /config/pull", Get().PullHandler())
	web.Handle("POST /config/cancel", Get().CancelHandler())
}
