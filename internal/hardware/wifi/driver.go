// Package wifi brings the radio up without any of Android's networking.
//
// The part is an AR6320 (QCA6174) over SDIO, driven by qcacld-2.0. Android loaded the driver,
// associated through wificond and wpa_supplicant, and did DHCP inside system_server — none of
// which exists after the takeover.
package wifi

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// fwpath is a module parameter with a setter: writing it calls kickstart_driver(), which runs
	// HDD init and downloads firmware to the chip over SDIO. It is also how Android unloaded the
	// driver when it decided to save power.
	fwpath = "/sys/module/wlan/parameters/fwpath"

	// conMode 0 is station mode.
	conMode = "/sys/module/wlan/parameters/con_mode"

	Interface = "wlan0"
)

// Loaded reports whether the interface exists.
func Loaded() bool {
	_, err := os.Stat("/sys/class/net/" + Interface)
	return err == nil
}

// Load brings the driver up, which takes about a second and a quarter. It is a no-op when the
// interface is already there.
func Load() error {
	if Loaded() {
		return nil
	}
	if _, err := os.Stat(fwpath); err != nil {
		return fmt.Errorf("no qcacld driver on this kernel: %w", err)
	}

	if err := os.WriteFile(conMode, []byte("0"), 0o644); err != nil {
		// Not fatal: the default is station mode anyway.
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

// MAC is the interface's address, which is the device's identity to Home Assistant.
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
