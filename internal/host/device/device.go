// Package device is adb, as the rest of lanovoctl uses it.
package device

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

// Binary is the adb executable, found on PATH.
const Binary = "adb"

// rcMarker carries a shell command's exit status back in its output, because `adb shell` returns
// adb's status, not the command's.
const rcMarker = "__lanovo_rc="

// ErrUnreachable means nothing is there for adb to drive.
var ErrUnreachable = errors.New("no device adb can drive")

// PlatformTools is where to get adb, for an error worth reading.
const PlatformTools = "https://developer.android.com/tools/releases/platform-tools"

// Require reports whether adb is available, and says what to install when it is not.
func Require() error {
	if _, err := exec.LookPath(Binary); err != nil {
		return fmt.Errorf("adb is not on PATH: install Android platform-tools and try again\n"+
			"  %s\n"+
			"  macOS: brew install --cask android-platform-tools\n"+
			"  Debian/Ubuntu: apt install android-sdk-platform-tools", PlatformTools)
	}
	return nil
}

// Device is one attached display.
type Device struct {
	serial string
}

// Info is a device as `adb devices` lists it.
type Info struct {
	Serial string
	State  string
	Model  string
}

func (i Info) String() string {
	if i.Model == "" {
		return i.Serial
	}
	return fmt.Sprintf("%s (%s)", i.Model, i.Serial)
}

// The adb states worth naming.
const (
	StateOnline   = "device"
	StateRecovery = "recovery"
)

// List is every online device.
func List() ([]Info, error) { return list(StateOnline) }

func list(states ...string) ([]Info, error) {
	out, err := run(context.Background(), "devices", "-l")
	if err != nil {
		return nil, err
	}

	var found []Info
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(line, "List of devices") {
			continue
		}
		info := Info{Serial: fields[0], State: fields[1]}
		for _, f := range fields[2:] {
			if v, ok := strings.CutPrefix(f, "model:"); ok {
				info.Model = v
			}
		}
		for _, want := range states {
			if info.State == want {
				found = append(found, info)
				break
			}
		}
	}
	return found, nil
}

// Connect opens a device by serial, or the only one attached when serial is empty.
func Connect(serial string) (*Device, error) {
	if err := Require(); err != nil {
		return nil, err
	}

	devices, err := List()
	if err != nil {
		return nil, err
	}
	switch {
	case len(devices) == 0:
		return nil, ErrUnreachable
	case serial == "" && len(devices) > 1:
		return nil, fmt.Errorf("%d devices attached — name one with --serial", len(devices))
	case serial == "":
		serial = devices[0].Serial
	}
	return &Device{serial: serial}, nil
}

// Serial is the device this acts on.
func (d *Device) Serial() string { return d.serial }

// Shell runs a command and returns its output, failing on a nonzero exit.
func (d *Device) Shell(cmd string) (string, error) {
	out, code, err := d.ShellCode(cmd)
	if err != nil {
		return out, err
	}
	if code != 0 {
		return out, fmt.Errorf("%s: exit %d: %s", cmd, code, strings.TrimSpace(out))
	}
	return out, nil
}

// ShellCode runs a command and returns its exit status rather than failing on it.
func (d *Device) ShellCode(cmd string) (string, int, error) {
	raw, err := d.run(context.Background(), "shell", cmd+"; echo "+rcMarker+"$?")
	if err != nil {
		return raw, -1, err
	}
	return splitRC(raw)
}

func splitRC(raw string) (string, int, error) {
	at := strings.LastIndex(raw, rcMarker)
	if at < 0 {
		return raw, -1, fmt.Errorf("no exit status in output: %q", raw)
	}
	body := strings.TrimRight(raw[:at], "\r\n")
	code, err := strconv.Atoi(strings.TrimSpace(raw[at+len(rcMarker):]))
	if err != nil {
		return body, -1, fmt.Errorf("unreadable exit status: %w", err)
	}
	return body, code, nil
}

// Getprop reads one property, empty when it is unset.
func (d *Device) Getprop(name string) (string, error) {
	out, err := d.Shell("getprop " + name)
	return strings.TrimSpace(out), err
}

// Setprop writes one property.
func (d *Device) Setprop(name, value string) error {
	_, err := d.Shell(fmt.Sprintf("setprop %s %s", name, quote(value)))
	return err
}

// IsRoot reports whether adbd is running as root, which most of the install needs.
func (d *Device) IsRoot() (bool, error) {
	out, _, err := d.ShellCode("id -u")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "0", nil
}

// Root restarts adbd as root. A no-op when already root.
func (d *Device) Root(ctx context.Context) error {
	if ok, err := d.IsRoot(); err == nil && ok {
		return nil
	}
	if _, err := d.run(ctx, "root"); err != nil {
		return err
	}
	// adbd goes away and comes back.
	time.Sleep(time.Second)
	if _, err := d.run(ctx, "wait-for-device"); err != nil {
		return err
	}

	ok, err := d.IsRoot()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("adb root did not take: adbd is still unprivileged")
	}
	return nil
}

// DisableVerity turns dm-verity off, reporting whether the device has to reboot for it to take.
//
// What adbd says in its reply, rather than an assumption: one that was already disabled needs no
// reboot.
func (d *Device) DisableVerity(ctx context.Context) (reboot bool, err error) {
	out, err := d.run(ctx, "disable-verity")
	if err != nil {
		return false, fmt.Errorf("disable-verity: %w", err)
	}

	said := strings.ToLower(out)
	switch {
	case strings.Contains(said, "already disabled"):
		return false, nil
	case strings.Contains(said, "now disabled"), strings.Contains(said, "reboot"):
		return true, nil
	}

	// Wording this build does not use. Reboot anyway: the cost is a restart, against an install
	// onto a read-only /system.
	return true, nil
}

// Exists reports whether a path is there.
func (d *Device) Exists(path string) (bool, error) {
	_, code, err := d.ShellCode("ls " + quote(path))
	if err != nil {
		return false, err
	}
	return code == 0, nil
}

// ReadFile reads a file off the device.
func (d *Device) ReadFile(path string) ([]byte, error) {
	out, err := d.Shell("cat " + quote(path))
	return []byte(out), err
}

// PushFile copies a local file over and sets its mode.
func (d *Device) PushFile(local, remote string, mode os.FileMode) error {
	if _, err := d.run(context.Background(), "push", local, remote); err != nil {
		return err
	}
	_, err := d.Shell(fmt.Sprintf("chmod %o %s", mode.Perm(), quote(remote)))
	return err
}

// PullFile copies a file back to the host.
func (d *Device) PullFile(remote, local string) error {
	_, err := d.run(context.Background(), "pull", remote, local)
	return err
}

// WriteFile creates a file on the device from bytes in hand.
func (d *Device) WriteFile(remote string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp("", "lanovo")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return d.PushFile(tmp.Name(), remote, mode)
}

// Reboot restarts the device. target is "" for a normal boot, or "bootloader"/"recovery".
func (d *Device) Reboot(target string) error {
	args := []string{"reboot"}
	if target != "" {
		args = append(args, target)
	}
	_, err := d.run(context.Background(), args...)
	return err
}

// WaitBooted blocks until the device is back and Android says it finished booting.
func (d *Device) WaitBooted(ctx context.Context) error {
	if _, err := d.run(ctx, "wait-for-device"); err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if prop.Booted(d) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (d *Device) run(ctx context.Context, args ...string) (string, error) {
	return run(ctx, append([]string{"-s", d.serial}, args...)...)
}

func run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, Binary, args...).CombinedOutput()
	if err != nil {
		return string(
				out,
			), fmt.Errorf(
				"adb %s: %w: %s",
				strings.Join(args, " "),
				err,
				strings.TrimSpace(string(out)),
			)
	}
	return string(out), nil
}

// quote makes a path safe to hand to the device's shell.
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
