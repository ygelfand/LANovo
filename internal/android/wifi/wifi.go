package wifi

import (
	"github.com/ygelfand/libcountertop/pkg/host/adb"
	"github.com/ygelfand/libcountertop/pkg/host/wifisetup"

	"github.com/ygelfand/LANovo/internal/layout"
)

const (
	iface   = layout.WifiIface
	sockets = layout.WifiSockets
)

func On(d *adb.Device) wifisetup.Supplicant {
	return wifisetup.Supplicant{Device: d, Sockets: sockets, Iface: iface}
}
