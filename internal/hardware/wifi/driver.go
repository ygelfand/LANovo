package wifi

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// Writing fwpath calls kickstart_driver(): HDD init and firmware download over SDIO.
	fwpath = "/sys/module/wlan/parameters/fwpath"

	// conMode 0 is station mode.
	conMode = "/sys/module/wlan/parameters/con_mode"

	Interface = "wlan0"
)

func Loaded() bool {
	_, err := os.Stat("/sys/class/net/" + Interface)
	return err == nil
}

func Load() error {
	if Loaded() {
		return nil
	}
	if _, err := os.Stat(fwpath); err != nil {
		return fmt.Errorf("no qcacld driver on this kernel: %w", err)
	}

	if err := os.WriteFile(conMode, []byte("0"), 0o644); err != nil {
		_ = err
	}
	if err := os.WriteFile(fwpath, []byte("sta"), 0o644); err != nil {
		return fmt.Errorf("loading the wlan driver: %w", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if Loaded() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("%s did not appear after loading the driver", Interface)
}

func MAC() (string, error) {
	b, err := os.ReadFile("/sys/class/net/" + Interface + "/address")
	if err != nil {
		return "", fmt.Errorf("reading the wlan address: %w", err)
	}
	mac := strings.TrimSpace(string(b))
	if mac == "" || mac == "00:00:00:00:00:00" {
		return "", fmt.Errorf("the wlan address reads %q", mac)
	}
	return mac, nil
}
