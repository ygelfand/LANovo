package takeover

import (
	"bytes"
	"fmt"

	"github.com/ygelfand/LANovo/internal/host/device"
)

// / is the system image on this device.
const DefaultProp = "/default.prop"

// adbd reads ro.secure once at start.
const Insecure = "ro.secure=0"

func MakeInsecure(d *device.Device) (changed bool, err error) {
	cur, err := d.ReadFile(DefaultProp)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", DefaultProp, err)
	}

	next, changed := setProp(cur, "ro.secure", "0")
	if !changed {
		return false, nil
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
