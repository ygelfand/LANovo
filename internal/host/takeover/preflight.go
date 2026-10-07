package takeover

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/host/device"
)

// decodeBase64 undoes the device-side base64 used to carry raw bytes through adb's text mode.
//
// Only line endings are dropped: anything else the device printed cannot be told apart from the
// payload, because dd's own summary is made of base64 characters. That is why every caller sends
// stderr to /dev/null, and why what comes back is checked for the boot magic before it is used.
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

// Check is one prerequisite and whether the device meets it.
type Check struct {
	Name string
	OK   bool

	// Got is what the device reported, Want what it needed to be.
	Got  string
	Want string

	// Fatal marks a check the install cannot proceed without.
	Fatal bool

	// Fix is what the person has to do about it. Empty when LANovo handles it.
	Fix string
}

// The firmware LANovo is developed against. Same image for amber and blueberry.
const (
	WantIncremental = "5307861"
	WantIoT         = "1.1.0"
)

// Preflight reads the device and reports what is and is not in order.
//
// It checks rather than fixes the prerequisites: unlocking the bootloader and flashing the debug
// firmware are done with fastboot before LANovo is involved.
func Preflight(d *device.Device) ([]Check, error) {
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
			// No Fix: the install turns it off itself.
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

func mustSlots(d *device.Device) []string {
	slots, err := Slots(d)
	if err != nil {
		return nil
	}
	return slots
}

// Verity is the name of the check DisableVerity clears.
const Verity = "dm-verity"

// Failed reports whether a named check is present and did not pass.
func Failed(checks []Check, name string) bool {
	for _, c := range checks {
		if c.Name == name {
			return !c.OK
		}
	}
	return false
}

// DisableVerity turns dm-verity off and brings the device back on the patched image.
//
// Before anything else, not at the end like the boot patch: /system cannot be written while verity
// is enforcing.
func DisableVerity(ctx context.Context, d *device.Device) (rebooted bool, err error) {
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

	// Root goes with the reboot, and everything after this needs it back.
	if err := d.Root(ctx); err != nil {
		return true, err
	}

	if mode, err := d.Getprop(prop.VerityMode); err == nil && mode != "disabled" {
		return true, fmt.Errorf("dm-verity is still %s after disabling it and rebooting", mode)
	}
	return true, nil
}

// Blocked is the fatal checks that failed.
func Blocked(checks []Check) []Check { return BlockedExcept(checks) }

// BlockedExcept is Blocked without the named checks, for the ones a caller is about to fix itself.
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
