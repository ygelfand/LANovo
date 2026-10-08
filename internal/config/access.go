package config

type Access struct {
	// ro.secure is 0: adb over TCP is an unauthenticated root shell.
	ADB bool `json:"adb"`
}

const ADBPort = 5555

func defaultAccess() Access { return Access{} }

type AccessWriter struct{ st *Store }

func (w AccessWriter) ADB(v bool) error {
	return w.st.Update(func(c *Config) { c.Access.ADB = v })
}
