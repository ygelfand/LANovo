package config

// Network is how the device reaches the rest of the world.
type Network struct {
	// Verify is whether a certificate has to check out before the device will download over it.
	//
	// On unless somebody turns it off. Off is for a device that cannot verify — one whose clock is
	// still in 1970 because nothing has set it, or a Home Assistant behind a certificate the
	// device has no way to trust — and it is meant to be a deliberate, visible choice rather than
	// a quiet default.
	Verify bool `json:"verify"`

	// Address is the last address DHCP gave the device, so a restart can ask for it back.
	//
	// RFC 2131 section 4.4.1: a client that had an address should name it when it asks for one, and
	// a server that can honor it will. Without this a reboot takes whatever is next in the pool,
	// which moves the device out from under anything pointed at it by address.
	//
	// A hint and nothing more. The server decides, the lease that comes back is what is used, and
	// an address that is no longer ours is simply not offered again.
	Address string `json:"address,omitempty"`
}

// DefaultVerify is on, and anything that turns it off should say so where it can be seen.
const DefaultVerify = true

func defaultNetwork() Network { return Network{Verify: DefaultVerify} }

// NetworkWriter records how the device reaches the rest of the world.
type NetworkWriter struct{ st *Store }

func (w NetworkWriter) Verify(v bool) error {
	return w.st.Update(func(c *Config) { c.Network.Verify = v })
}

// Address remembers the address to ask for next time. An empty string forgets it, which is what a
// device that could not get one should do rather than keep asking for an address it never had.
func (w NetworkWriter) Address(v string) error {
	return w.st.Update(func(c *Config) { c.Network.Address = v })
}
