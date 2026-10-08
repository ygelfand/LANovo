package component

import (
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/config"
)

type Registry = sharedcomponent.Registry[config.Config]

func New() *Registry { return sharedcomponent.New[config.Config](nil) }

var shared = New()

func Default() *Registry { return shared }

func Register[T sharedcomponent.Component](
	p sharedcomponent.Phase,
	make func() T,
	opts ...sharedcomponent.Option,
) {
	sharedcomponent.Register(shared, p, make, opts...)
}
