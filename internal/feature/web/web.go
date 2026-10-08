package web

import (
	"net/http"
	"sync"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	libweb "github.com/ygelfand/libcountertop/pkg/runtime/web"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() {
	component.Register(sharedcomponent.Network, Get, sharedcomponent.Order(50))
}

const Port = 80

var get = sync.OnceValue(func() *libweb.Server {
	return libweb.New(
		libweb.Options{Port: Port, APIPort: layout.Port, Adopted: Adopted, Page: page},
	)
})

func Get() *libweb.Server { return get() }

func Handle(pattern string, h http.Handler) { Get().Handle(pattern, h) }

func Adopted() bool { return config.Get().API.Adopted }
