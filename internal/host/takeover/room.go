package takeover

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/host/device"
	"github.com/ygelfand/LANovo/internal/layout"
)

// RoomMargin is what an install leaves behind on /.
//
// Filling a filesystem to the last block is its own fault to have later, and / holds the init
// script and the name file as well as the binary. It is small because the margin is not the point:
// the point is finding out before anything is written rather than partway through.
const RoomMargin = 4 << 20

// Room reports whether a lanovod of need bytes will fit on /.
//
// The binary already there counts as room, because adb push truncates the destination before it
// writes: replacing a 21MB binary with a 22MB one needs one more megabyte, not twenty-two.
func Room(d *device.Device, need int64) (Check, error) {
	free, err := freeOnRoot(d)
	if err != nil {
		return Check{}, err
	}
	existing, err := sizeOf(d, layout.Binary)
	if err != nil {
		return Check{}, err
	}

	have := free + existing
	want := need + RoomMargin

	return Check{
		Name:  "room",
		Got:   megabytes(have) + " free",
		Want:  megabytes(want),
		OK:    have >= want,
		Fatal: true,
		Fix:   "/ is full — remove something before installing",
	}, nil
}

// freeOnRoot is how many bytes / has left.
//
// The free count rather than the one available to an unprivileged process: the install runs as
// root, and root may write into the blocks ext4 holds back.
func freeOnRoot(d *device.Device) (int64, error) {
	out, err := d.Shell(`stat -f -c "%f %S" /`)
	if err != nil {
		return 0, fmt.Errorf("reading free space on /: %w", err)
	}
	return parseFree(out)
}

func parseFree(out string) (int64, error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, fmt.Errorf("unreadable free space: %q", out)
	}

	blocks, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unreadable free block count in %q: %w", out, err)
	}
	size, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unreadable block size in %q: %w", out, err)
	}
	if blocks < 0 || size <= 0 {
		return 0, fmt.Errorf("nonsense free space: %q", out)
	}
	return blocks * size, nil
}

// sizeOf is how many bytes the file at path takes. A file that is not there is no bytes, which is
// the answer a first install wants rather than an error.
func sizeOf(d *device.Device, path string) (int64, error) {
	out, code, err := d.ShellCode(`stat -c "%s" ` + path)
	if err != nil {
		return 0, err
	}
	if code != 0 {
		return 0, nil
	}
	return parseSize(out)
}

func parseSize(out string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unreadable file size: %q", out)
	}
	return n, nil
}

// megabytes is a size as somebody reading an installer would write it.
func megabytes(b int64) string { return fmt.Sprintf("%.1f MB", float64(b)/(1<<20)) }
