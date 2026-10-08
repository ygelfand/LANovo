package wifi

import (
	"fmt"
	"os"
	"sync"
	"time"

	sharedwifi "github.com/ygelfand/libcountertop/pkg/network/wifi"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/layout"
)

const (
	// Writing fwpath calls kickstart_driver(): HDD init and firmware download over SDIO.
	fwpath = "/sys/module/wlan/parameters/fwpath"

	// conMode 0 is station mode.
	conMode = "/sys/module/wlan/parameters/con_mode"
)

func init() {
	component.Register(sharedcomponent.Hardware, Get, sharedcomponent.Order(20),
		sharedcomponent.Supervise(service.Restart(2*time.Second, time.Minute)))
}

var get = sync.OnceValue(func() *sharedwifi.Radio {
	return sharedwifi.New(sharedwifi.Options{
		Interface:  layout.WifiIface,
		SocketDir:  layout.WifiSockets,
		ConfigPath: layout.WifiConf,
		Bgscan:     layout.WifiBgscan,
		User:       layout.WifiUser,
		Client:     "lanovod",
		Load:       load,
	})
})

func Get() *sharedwifi.Radio { return get() }

func load() error {
	if _, err := os.Stat(fwpath); err != nil {
		return fmt.Errorf("no qcacld driver on this kernel: %w", err)
	}
	_ = os.WriteFile(conMode, []byte("0"), 0o644)
	if err := os.WriteFile(fwpath, []byte("sta"), 0o644); err != nil {
		return fmt.Errorf("loading the wlan driver: %w", err)
	}
	return nil
}
