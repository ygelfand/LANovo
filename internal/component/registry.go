package component

import (
	sharedlib "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/config"
)

type Option = sharedlib.Option

var Order = sharedlib.Order
var Supervise = sharedlib.Supervise

type Registry = sharedlib.Registry[config.Config]

func New() *Registry { return sharedlib.New[config.Config](nil) }

var shared = New()

func Default() *Registry { return shared }
func Register[T Component](p Phase, make func() T, opts ...Option) {
	shared.Add(p, func() Component { return make() }, opts...)
}
