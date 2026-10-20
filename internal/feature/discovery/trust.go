package discovery

import (
	"crypto/tls"
	"log/slog"
	"net/http"
	"sync"

	sharedpeer "github.com/ygelfand/libcountertop/pkg/network/peer"
	"github.com/ygelfand/libcountertop/pkg/network/trust"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/network"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/layout"
)

var Trust = sync.OnceValue(func() *trust.Trust {
	cert, err := trust.LoadIdentity(layout.StateDir + "/peer.pem")
	if err != nil {
		slog.Error("loading the peer certificate failed", "err", err)
	}
	t, err := trust.New(trust.Options{
		ID:       ID(),
		Identity: cert,
		Key:      network.Get().HomeKey,
		Peers:    func() []sharedpeer.Peer { return Get().Peers() },
		Changed:  func() { Get().Changed.Emit(struct{}{}) },
	})
	if err != nil {
		panic(err)
	}
	Get().Found.Listen(t.Found)
	Get().Lost.Listen(t.Lost)
	network.Get().Changed.Listen(func(schema.Network) { t.Rekey() })
	return t
})

func init() {
	component.Register(sharedcomponent.Network, Trust, sharedcomponent.Order(61))
	web.Get().Secure(web.SecurePort, func() *tls.Config { return Trust().ServerConfig() })
	web.HandlePeer(
		"POST "+trust.ConfirmPath,
		lazy(func() http.Handler { return Trust().ConfirmHandler() }),
	)
	web.HandlePeer(
		"POST "+trust.ProvePath,
		lazy(func() http.Handler { return Trust().ProveHandler() }),
	)
}

func Home(h http.Handler) http.Handler {
	return lazy(func() http.Handler { return Trust().Home(h) })
}

// TODO: remove with the plain-HTTP peer routes once no device predates the secure port.
func Legacy(h http.Handler) http.Handler {
	return lazy(func() http.Handler { return Trust().Legacy(h) })
}

func Client() *http.Client {
	return &http.Client{
		Transport: transport(func() http.RoundTripper { return Trust().Client(0).Transport }),
	}
}

func AnyClient() *http.Client {
	return &http.Client{
		Transport: transport(func() http.RoundTripper { return Trust().AnyClient(0).Transport }),
	}
}

type judge struct{}

func (judge) Standing(id string) sharedpeer.Standing { return Trust().Standing(id) }

type lazy func() http.Handler

func (l lazy) ServeHTTP(w http.ResponseWriter, r *http.Request) { l().ServeHTTP(w, r) }

type transport func() http.RoundTripper

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t().RoundTrip(r) }
