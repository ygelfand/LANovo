package wifi

import (
	"fmt"
	"strings"
	"time"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/layout"
)

const (
	Supplicant = layout.Supplicant
	ConfigPath = layout.WifiConf

	// update_config makes the supplicant write networks back itself.
	config = "ctrl_interface=" + sockets + "\nupdate_config=1\nbgscan=\"" + layout.WifiBgscan + "\"\n"
)

func Up(d *adb.Device, p layout.Paths) error {
	has, err := d.Exists(p.Binary)
	if err != nil {
		return fmt.Errorf("wifi: %w", err)
	}
	if !has {
		return fmt.Errorf("wifi: run lanovo install first")
	}
	if err := loadDriver(d); err != nil {
		return err
	}
	if _, err := d.Shell("ip link set " + iface + " up"); err != nil {
		return fmt.Errorf("wifi: bringing %s up: %w", iface, err)
	}
	return startSupplicant(d, p)
}

// Writing fwpath calls kickstart_driver(): driver init and firmware download over SDIO.
func loadDriver(d *adb.Device) error {
	if out, _ := d.Shell(
		"ls /sys/class/net/" + iface + " 2>/dev/null",
	); strings.TrimSpace(
		out,
	) != "" {
		return nil
	}

	if _, err := d.Shell("echo sta > /sys/module/wlan/parameters/fwpath"); err != nil {
		return fmt.Errorf("wifi: loading the driver: %w", err)
	}

	for range 30 {
		if out, _ := d.Shell(
			"ls /sys/class/net/" + iface + " 2>/dev/null",
		); strings.TrimSpace(
			out,
		) != "" {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("wifi: %s did not appear after loading the driver", iface)
}

func startSupplicant(d *adb.Device, p layout.Paths) error {
	if On(d, p).Answering() {
		return nil
	}

	if _, err := d.Shell("mkdir -p " + sockets); err != nil {
		return fmt.Errorf("wifi: %w", err)
	}

	has, err := d.Exists(ConfigPath)
	if err != nil {
		return fmt.Errorf("wifi: %w", err)
	}
	if !has {
		if err := d.WriteFile(ConfigPath, []byte(config), 0o660); err != nil {
			return fmt.Errorf("wifi: writing %s: %w", ConfigPath, err)
		}
	}

	up := running(d)
	cmds := []string{"chown wifi:wifi " + ConfigPath, "chmod 660 " + ConfigPath}
	if !up {
		cmds = append([]string{"rm -f " + sockets + "/" + iface}, cmds...)
	}
	for _, cmd := range cmds {
		if _, err := d.Shell(cmd); err != nil {
			return fmt.Errorf("wifi: %w", err)
		}
	}

	if !up {
		if err := start(d); err != nil {
			return err
		}
	}

	for range 20 {
		if On(d, p).Answering() {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("wifi: the supplicant did not answer on %s", sockets)
}

func running(d *adb.Device) bool {
	state, _ := d.Shell("getprop init.svc." + layout.SupplicantService)
	return strings.TrimSpace(state) == "running"
}

// init reads InitRC only at boot.
func start(d *adb.Device) error {
	if known, _ := d.Shell(
		"getprop init.svc." + layout.SupplicantService,
	); strings.TrimSpace(
		known,
	) != "" {
		_, err := d.Shell("setprop ctl.start " + layout.SupplicantService)
		return err
	}

	cmd := fmt.Sprintf("%s -B -i %s -Dnl80211 -c %s", Supplicant, iface, ConfigPath)
	if _, err := d.Shell(cmd); err != nil {
		return fmt.Errorf("wifi: starting the supplicant: %w", err)
	}
	return nil
}
