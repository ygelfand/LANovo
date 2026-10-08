package takeover

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

func decodeBase64(s string) ([]byte, error) {
	var clean strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '+',
			r == '/',
			r == '=':
			clean.WriteRune(r)
		}
	}
	return base64.StdEncoding.DecodeString(clean.String())
}

type Check struct {
	Name string
	OK   bool

	Got  string
	Want string

	Fatal bool

	Fix string
}

const (
	WantIncremental = "5307861"
	WantIoT         = "1.1.0"
)

func Preflight(d *adb.Device) ([]Check, error) {
	props := map[string]string{}
	for _, p := range []string{
		prop.Model,
		"ro.product.device",
		"ro.build.type",
		"ro.build.version.incremental",
		"ro.build.version.iot",
		"ro.boot.vbmeta.device_state",
		prop.VerityMode,
		"ro.debuggable",
		"ro.secure",
		prop.SlotSuffix,
	} {
		v, err := d.Getprop(p)
		if err != nil {
			return nil, err
		}
		props[p] = v
	}

	root, err := d.IsRoot()
	if err != nil {
		return nil, err
	}

	checks := []Check{
		{
			Name:  "model",
			Got:   props[prop.Model],
			Want:  "a Lenovo Smart Display",
			OK:    strings.Contains(props[prop.Model], "Lenovo Smart Display"),
			Fatal: true,
			Fix:   "this is not a device LANovo knows",
		},
		{
			Name:  "bootloader",
			Got:   props["ro.boot.vbmeta.device_state"],
			Want:  "unlocked",
			OK:    props["ro.boot.vbmeta.device_state"] == "unlocked",
			Fatal: true,
			Fix:   "unlock with fastboot — see the XDA thread in the README",
		},
		{
			Name:  Verity,
			Got:   props[prop.VerityMode],
			Want:  "disabled",
			OK:    props[prop.VerityMode] == "disabled",
			Fatal: true,
		},
		{
			Name:  "build type",
			Got:   props["ro.build.type"],
			Want:  "userdebug",
			OK:    props["ro.build.type"] == "userdebug",
			Fatal: true,
			Fix:   "flash the Android 8.1 Debug Firmware — a user build cannot boot permissive",
		},
		{
			Name: "firmware",
			Got:  props["ro.build.version.incremental"] + " / Things " + props["ro.build.version.iot"],
			Want: WantIncremental + " / Things " + WantIoT,
			OK: props["ro.build.version.incremental"] == WantIncremental &&
				props["ro.build.version.iot"] == WantIoT,
			Fix: "a different build — LANovo is developed against this one",
		},
		{
			Name:  "adb root",
			Got:   fmt.Sprintf("%v", root),
			Want:  "true",
			OK:    root,
			Fatal: true,
			Fix:   "adb root",
		},
	}

	for _, slot := range mustSlots(d) {
		b, err := ReadBoot(d, slot)
		if err != nil {
			checks = append(checks, Check{
				Name: "permissive boot" + slot, Got: err.Error(), Want: PermissiveArg,
			})
			continue
		}
		checks = append(checks, Check{
			Name: "permissive boot" + slot,
			Got:  map[bool]string{true: "patched", false: "not patched"}[b.Permissive],
			Want: "patched",
			OK:   b.Permissive,
			Fix:  "lanovoctl install patches it",
		})
	}

	return checks, nil
}

func mustSlots(d *adb.Device) []string {
	slots, err := Slots(d)
	if err != nil {
		return nil
	}
	return slots
}

const Verity = "dm-verity"

func Failed(checks []Check, name string) bool {
	for _, c := range checks {
		if c.Name == name {
			return !c.OK
		}
	}
	return false
}

// /system cannot be written while verity is enforcing.
func DisableVerity(ctx context.Context, d *adb.Device) (rebooted bool, err error) {
	reboot, err := d.DisableVerity(ctx)
	if err != nil {
		return false, err
	}
	if !reboot {
		return false, nil
	}

	if err := d.Reboot(""); err != nil {
		return false, err
	}
	if err := d.WaitBooted(ctx); err != nil {
		return true, err
	}

	if err := d.Root(ctx); err != nil {
		return true, err
	}

	if mode, err := d.Getprop(prop.VerityMode); err == nil && mode != "disabled" {
		return true, fmt.Errorf("dm-verity is still %s after disabling it and rebooting", mode)
	}
	return true, nil
}

func Blocked(checks []Check) []Check { return BlockedExcept(checks) }

func BlockedExcept(checks []Check, skip ...string) []Check {
	var out []Check
	for _, c := range checks {
		if !c.Fatal || c.OK || slices.Contains(skip, c.Name) {
			continue
		}
		out = append(out, c)
	}
	return out
}
