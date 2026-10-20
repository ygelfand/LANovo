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
		Store:  &component.Settings,
		Peers:  discovery.Get(),
		Port:   web.Port,
		Secure: web.SecurePort,
		Client: discovery.AnyClient(),
		Stage:  syncview.NewStage(shell.Get()),
	})
})

func init() {
	web.HandlePeer("POST /config/offer", Get().OfferHandler())
	web.HandlePeer("POST /config/ask", Get().AskHandler())
	web.HandlePeer("POST /config/pull", Get().PullHandler())
	web.HandlePeer("POST /config/cancel", Get().CancelHandler())
	// TODO: remove the plain-HTTP sync routes once no device predates the secure port.
	web.Handle("POST /config/offer", discovery.Legacy(Get().OfferHandler()))
	web.Handle("POST /config/ask", discovery.Legacy(Get().AskHandler()))
	web.Handle("POST /config/pull", discovery.Legacy(Get().PullHandler()))
	web.Handle("POST /config/cancel", discovery.Legacy(Get().CancelHandler()))
}
