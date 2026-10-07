package dhcp

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/layout"
	shared "github.com/ygelfand/libcountertop/pkg/network/dhcp"
)

type Lease = shared.Lease

var Probe = shared.Probe

// hostname is what the device asks the server to call it.
//
// The name someone gave it, slugged the way the ESPHome node name is, so the device answers to one
// name on the network however it is looked up. Not the kernel's hostname: this board's is
// localhost and nothing sets it.
func hostname() string {
	if slug := layout.Slug(config.Get().Device.Name); slug != "" {
		return slug
	}
	return Fallback
}

// Fallback is the hostname a device with no name of its own asks for.
const Fallback = "lanovo"
