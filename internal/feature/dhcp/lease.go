package dhcp

import (
	shared "github.com/ygelfand/libcountertop/pkg/network/dhcp"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/layout"
)

type Lease = shared.Lease

var Probe = shared.Probe

func hostname() string {
	if slug := layout.Slug(config.Get().Device.Name); slug != "" {
		return slug
	}
	return Fallback
}

const Fallback = "lanovo"
