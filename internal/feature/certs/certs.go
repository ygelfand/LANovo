package certs

import (
	"crypto/tls"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/network/certwarden"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/layout"
)

type componentCerts struct{ *certwarden.Warden }

func (c componentCerts) Restore(cfg config.Config) { c.Warden.Restore(cfg.Certs) }

var get = sync.OnceValue(func() *certwarden.Warden {
	w := certwarden.New(certwarden.Options{
		Settings: config.CertsSection,
		Path:     layout.StateDir + "/https.pem",
		DeviceID: component.DeviceFrontend,
	})
	component.Settings.Add(w.Controls())
	return w
})

func Get() *certwarden.Warden { return get() }

func init() {
	component.Register(
		sharedcomponent.Network,
		func() componentCerts { return componentCerts{Get()} },
	)
	web.Get().Public(web.PublicPort, serverConfig)
}

func serverConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			if c, ok := Get().Certificate(); ok {
				return c, nil
			}
			return discovery.Trust().Identity(), nil
		},
	}
}
