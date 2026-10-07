// Package component binds the shared registry to product configuration and board selection.
package component

import (
	"github.com/ygelfand/LANovo/internal/config"
	sharedlib "github.com/ygelfand/libcountertop/pkg/runtime/component"
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
