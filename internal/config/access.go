package config

// Access is what the device leaves open to the network.
type Access struct {
	// ADB is whether adbd listens on TCP; ro.secure is 0, so that is an unauthenticated root shell.
	ADB bool `json:"adb"`

	// Control is whether the control socket listens.
	Control bool `json:"control"`
}

// ADBPort is where adbd listens when it is turned on, which is the port adb itself tries.
const ADBPort = 5555

func defaultAccess() Access { return Access{Control: true} }

// AccessWriter records what the device leaves open.
type AccessWriter struct{ st *Store }

func (w AccessWriter) ADB(v bool) error {
	return w.st.Update(func(c *Config) { c.Access.ADB = v })
}

func (w AccessWriter) Control(v bool) error {
	return w.st.Update(func(c *Config) { c.Access.Control = v })
}
