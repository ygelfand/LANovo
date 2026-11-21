package takeover

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/parts"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

const recoverySystemMount = "/system_root"

type binPatch struct {
	offset int64
	before []byte
	after  []byte
}

var cubeAdbd = []binPatch{
	{0x1b92c, []byte{0x2d, 0xe9, 0xf0, 0x4f}, []byte{0x01, 0x20, 0x70, 0x47}},
	{0x038aa, []byte{0x46, 0xf0, 0x3f, 0xff}, []byte{0xaf, 0xf3, 0x00, 0x80}},
	{0x038b4, []byte{0x46, 0xf0, 0x8c, 0xfe}, []byte{0xaf, 0xf3, 0x00, 0x80}},
	{0x038be, []byte{0x46, 0xf0, 0x5f, 0xfe}, []byte{0xaf, 0xf3, 0x00, 0x80}},
}

var cubeProps = map[string]string{
	"ro.secure":              "0",
	"ro.adb.secure":          "0",
	"ro.debuggable":          "1",
	"persist.sys.usb.config": "adb",
	prop.Board:               "cube",
}

func PrepareRecovery(d *adb.Device) (Result, error) {
	var res Result

	slot, err := d.Getprop(prop.SlotSuffix)
	if err != nil {
		return res, err
	}
	dir, err := ByName(d)
	if err != nil {
		return res, err
	}
	part := dir + "/system" + slot

	if err := mountRecoverySystem(d, part); err != nil {
		return res, err
	}
	defer func() {
		_, _ = d.Shell("sync")
		_, _ = d.Shell("toybox mount -o remount,ro " + recoverySystemMount)
		_, _ = d.Shell("toybox blockdev --setro " + part)
	}()

	adbdNote, err := patchBinary(d, recoverySystemMount+"/system/bin/adbd", cubeAdbd)
	if err != nil {
		return res, err
	}
	res.Steps = append(res.Steps, Step{What: "adbd", Note: adbdNote})

	if err := editProps(d, recoverySystemMount+"/default.prop", cubeProps); err != nil {
		return res, err
	}
	res.Steps = append(res.Steps, Step{What: "default.prop", Note: "root, adb and board set"})

	paths := board.Cube.Paths()
	rc, err := parts.RC(paths, board.Cube.HasCamera())
	if err != nil {
		return res, err
	}
	if err := installFile(d, recoverySystemMount+paths.InitRC, rc, 0o644); err != nil {
		return res, err
	}
	res.Steps = append(res.Steps, Step{What: "lanovod.rc", Note: paths.InitRC})

	return res, nil
}

func installFile(d *adb.Device, path string, data []byte, mode os.FileMode) error {
	const staged = "/tmp/lanovo-stage"
	if err := d.WriteFile(staged, data, mode); err != nil {
		return err
	}
	defer func() { _, _ = d.Shell("rm -f " + staged) }()
	if _, err := d.Shell("mkdir -p " + path[:strings.LastIndex(path, "/")]); err != nil {
		return err
	}
	if _, err := d.Shell(fmt.Sprintf("cat %s > %s", staged, path)); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	// init's ReadFile refuses a group- or world-writable .rc.
	if _, err := d.Shell(fmt.Sprintf("chmod %o %s", mode.Perm(), path)); err != nil {
		return fmt.Errorf("setting the mode on %s: %w", path, err)
	}
	return nil
}

func mountRecoverySystem(d *adb.Device, part string) error {
	if _, err := d.Shell("toybox blockdev --setrw " + part); err != nil {
		return fmt.Errorf("clearing the read-only flag on %s: %w", part, err)
	}
	if _, err := d.Shell("toybox mount -o remount,rw " + recoverySystemMount); err != nil {
		return fmt.Errorf("remounting %s read-write: %w", recoverySystemMount, err)
	}

	out, err := d.Shell("grep -E ' " + recoverySystemMount + " ' /proc/mounts")
	if err != nil {
		return err
	}
	if fields := strings.Fields(out); len(fields) < 4 || !strings.HasPrefix(fields[3], "rw") {
		return fmt.Errorf("%s did not become writable: %q", recoverySystemMount, out)
	}
	return nil
}

func patchBinary(d *adb.Device, path string, patches []binPatch) (string, error) {
	applied := 0
	for _, p := range patches {
		cur, err := peek(d, path, p.offset, len(p.before))
		if err != nil {
			return "", err
		}
		switch {
		case string(cur) == string(p.after):
			continue
		case string(cur) != string(p.before):
			return "", fmt.Errorf("%s at %d reads %x, neither stock nor ours", path, p.offset, cur)
		}
		if err := poke(d, path, p.offset, p.after); err != nil {
			return "", err
		}
		applied++
	}
	if applied == 0 {
		return "already patched", nil
	}
	return fmt.Sprintf("%d of %d edits applied", applied, len(patches)), nil
}

func editProps(d *adb.Device, path string, props map[string]string) error {
	cur, err := d.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	next := cur
	for key, value := range props {
		next, _ = setProp(next, key, value)
	}
	const staged = "/tmp/lanovo-prop"
	if err := d.WriteFile(staged, next, 0o600); err != nil {
		return err
	}
	defer func() { _, _ = d.Shell("rm -f " + staged) }()
	if _, err := d.Shell(fmt.Sprintf("cat %s > %s", staged, path)); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func peek(d *adb.Device, path string, at int64, n int) ([]byte, error) {
	raw, err := d.Shell(fmt.Sprintf(
		"toybox dd if=%s bs=1 skip=%d count=%d 2>/dev/null | od -An -tx1 | tr -d ' \\n'",
		path, at, n,
	))
	if err != nil {
		return nil, err
	}
	got, err := hex.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("reading %s at %d: %q", path, at, raw)
	}
	if len(got) != n {
		return nil, fmt.Errorf("read %d bytes of %s at %d, want %d", len(got), path, at, n)
	}
	return got, nil
}

func poke(d *adb.Device, path string, at int64, b []byte) error {
	var escaped strings.Builder
	for _, c := range b {
		fmt.Fprintf(&escaped, `\%03o`, c)
	}
	_, err := d.Shell(fmt.Sprintf(
		"printf '%s' | toybox dd of=%s bs=1 seek=%d conv=notrunc 2>/dev/null",
		escaped.String(), path, at,
	))
	return err
}
