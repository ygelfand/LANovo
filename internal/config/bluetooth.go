package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Bluetooth = schema.Bluetooth

const DefaultBluetoothProxy = schema.DefaultBluetoothProxy

var defaultBluetooth = schema.DefaultBluetooth

type BluetoothWriter struct{ st *Store }

func (w BluetoothWriter) Proxy(v bool) error {
	return w.st.Update(func(c *Config) { c.Bluetooth.Proxy = v })
}

func (w BluetoothWriter) Speaker(v bool) error {
	return w.st.Update(func(c *Config) { c.Bluetooth.Speaker = v })
}
