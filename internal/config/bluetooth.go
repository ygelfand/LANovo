package config

// Bluetooth is what the radio is allowed to do on somebody's behalf.
//
// Only whether, not how. Active scanning — asking advertisers for a scan response, which means
// transmitting rather than only listening — is Home Assistant's to request through the proxy's own
// mode, and a setting beside it here would be a second master for the same thing.
type Bluetooth struct {
	// Proxy is whether Home Assistant may use this device to hear Bluetooth it cannot reach itself.
	Proxy bool `json:"proxy"`

	// Speaker is whether a phone may connect and play to it.
	//
	// Not at the same time as the proxy: the controller has one reader, so whichever claims the
	// line first has it and the other is refused. Off by default for that reason — a device that
	// stopped being a proxy because it became a speaker would be a surprise.
	Speaker bool `json:"speaker"`
}

// On by default. A proxy is only useful to a house that has one, and a device that has to be found
// in the settings before it does the thing it is for mostly does not do it.
func defaultBluetooth() Bluetooth { return Bluetooth{Proxy: true} }

// BluetoothWriter changes what the radio may do.
type BluetoothWriter struct{ st *Store }

func (w BluetoothWriter) Proxy(v bool) error {
	return w.st.Update(func(c *Config) { c.Bluetooth.Proxy = v })
}

func (w BluetoothWriter) Speaker(v bool) error {
	return w.st.Update(func(c *Config) { c.Bluetooth.Speaker = v })
}
