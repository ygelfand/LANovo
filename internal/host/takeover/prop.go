package takeover

import (
	"bytes"
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
)

// / is the system image on this device.
const DefaultProp = "/default.prop"

// adbd reads ro.secure once at start.
const Insecure = "ro.secure=0"

func configureDefaultProp(d *adb.Device, b board.Board) (changed bool, err error) {
	cur, err := d.ReadFile(DefaultProp)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", DefaultProp, err)
	}

	next := cur
	if !b.Paths().X501F {
		next, changed = setProp(cur, "ro.secure", "0")
	}
	next, named := setProp(next, prop.Board, b.Name)
	if !named && !changed {
		return false, nil
	}
	if b.Paths().X501F {
		if _, err := backupOEMInit(d, DefaultProp, cur); err != nil {
			return false, err
		}
		if err := stageInstallFile(d, DefaultProp, next, 0o600); err != nil {
			return false, fmt.Errorf("writing %s: %w", DefaultProp, err)
		}
		return true, nil
	}
	if err := d.WriteFile(DefaultProp, next, 0o600); err != nil {
		return false, fmt.Errorf("writing %s: %w", DefaultProp, err)
	}
	return true, nil
}

func setProp(file []byte, key, value string) (out []byte, changed bool) {
	want := []byte(key + "=" + value)
	prefix := []byte(key + "=")

	lines := bytes.Split(file, []byte("\n"))
	for i, line := range lines {
		if !bytes.HasPrefix(line, prefix) {
			continue
		}
		if bytes.Equal(line, want) {
			return file, false
		}
		lines[i] = want
		return bytes.Join(lines, []byte("\n")), true
	}

	trailing := bytes.HasSuffix(file, []byte("\n"))
	if !trailing {
		file = append(file, '\n')
	}
	return append(file, append(want, '\n')...), true
}
