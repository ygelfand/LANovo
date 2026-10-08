package takeover

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/host/adb"
)

// init honors androidboot.selinux on any build that is not `user`.
const PermissiveArg = "androidboot.selinux=permissive"

// Android boot image header: cmdline is 512 bytes at offset 64.
const (
	bootMagic     = "ANDROID!"
	cmdlineOffset = 64
	cmdlineSize   = 512
	headerRead    = 4096
)

type Boot struct {
	Slot       string
	Partition  string
	PageSize   uint32
	Cmdline    string
	Permissive bool
}

func ReadBoot(d *adb.Device, slot string) (*Boot, error) {
	dir, err := ByName(d)
	if err != nil {
		return nil, err
	}
	part := dir + "/boot" + slot

	// adb's text mode mangles raw bytes.
	out, err := d.Shell(
		fmt.Sprintf("dd if=%s bs=%d count=1 2>/dev/null | base64", part, headerRead),
	)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", part, err)
	}
	head, err := decodeBase64(out)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", part, err)
	}
	if len(head) < headerRead {
		return nil, fmt.Errorf("%s: read %d bytes, wanted %d", part, len(head), headerRead)
	}

	pageSize, cmdline, err := header(head)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", part, err)
	}

	return &Boot{
		Slot:       slot,
		Partition:  part,
		PageSize:   pageSize,
		Cmdline:    cmdline,
		Permissive: strings.Contains(cmdline, PermissiveArg),
	}, nil
}

func header(head []byte) (pageSize uint32, cmdline string, err error) {
	if len(head) < cmdlineOffset+cmdlineSize {
		return 0, "", fmt.Errorf(
			"read %d bytes, wanted at least %d",
			len(head),
			cmdlineOffset+cmdlineSize,
		)
	}
	if !bytes.HasPrefix(head, []byte(bootMagic)) {
		return 0, "", fmt.Errorf("does not start with %q", bootMagic)
	}

	pageSize = binary.LittleEndian.Uint32(head[36:40])
	if pageSize == 0 || pageSize > headerRead {
		return 0, "", fmt.Errorf("implausible page size %d", pageSize)
	}

	return pageSize, string(
		bytes.TrimRight(head[cmdlineOffset:cmdlineOffset+cmdlineSize], "\x00"),
	), nil
}

func permissive(cmdline string) (string, error) {
	want := cmdline + " " + PermissiveArg

	// The cmdline field is NUL terminated.
	if len(want) >= cmdlineSize {
		return "", fmt.Errorf(
			"cmdline needs %d bytes, the field holds %d",
			len(want)+1,
			cmdlineSize,
		)
	}
	return want, nil
}

// The header id hashes the kernel and ramdisk, not the cmdline.
func writeCmdline(page []byte, want string) error {
	if len(page) < cmdlineOffset+cmdlineSize {
		return fmt.Errorf(
			"page is %d bytes, wanted at least %d",
			len(page),
			cmdlineOffset+cmdlineSize,
		)
	}

	if !bytes.HasPrefix(page, []byte(bootMagic)) {
		return fmt.Errorf("page does not start with %q", bootMagic)
	}
	if len(want) >= cmdlineSize {
		return fmt.Errorf("cmdline needs %d bytes, the field holds %d", len(want)+1, cmdlineSize)
	}

	field := page[cmdlineOffset : cmdlineOffset+cmdlineSize]
	clear(field)
	copy(field, want)
	return nil
}

func MakePermissive(d *adb.Device, slot string) (changed bool, err error) {
	b, err := ReadBoot(d, slot)
	if err != nil {
		return false, err
	}
	if b.Permissive {
		return false, nil
	}

	want, err := permissive(b.Cmdline)
	if err != nil {
		return false, fmt.Errorf("boot%s: %w", slot, err)
	}

	out, err := d.Shell(
		fmt.Sprintf("dd if=%s bs=%d count=1 2>/dev/null | base64", b.Partition, b.PageSize),
	)
	if err != nil {
		return false, err
	}
	page, err := decodeBase64(out)
	if err != nil {
		return false, err
	}
	if uint32(len(page)) != b.PageSize {
		return false, fmt.Errorf(
			"boot%s: read %d bytes of page, wanted %d",
			slot,
			len(page),
			b.PageSize,
		)
	}

	if err := writeCmdline(page, want); err != nil {
		return false, fmt.Errorf("boot%s: %w", slot, err)
	}

	if _, err := d.Shell("blockdev --setrw " + b.Partition); err != nil {
		return false, fmt.Errorf("clearing the read-only flag on %s: %w", b.Partition, err)
	}

	const tmp = "/data/local/tmp/lanovo-boot-page"
	if err := d.WriteFile(tmp, page, 0o600); err != nil {
		return false, err
	}
	defer d.Shell("rm -f " + tmp)

	if _, err := d.Shell(
		fmt.Sprintf("dd if=%s of=%s bs=%d count=1 2>/dev/null; sync", tmp, b.Partition, b.PageSize),
	); err != nil {
		return false, fmt.Errorf("writing %s: %w", b.Partition, err)
	}

	after, err := ReadBoot(d, slot)
	if err != nil {
		return false, err
	}
	if !after.Permissive {
		return false, fmt.Errorf("boot%s: the cmdline did not take", slot)
	}
	return true, nil
}

func Slots(d *adb.Device) ([]string, error) {
	dir, err := ByName(d)
	if err != nil {
		return nil, err
	}
	out, err := d.Shell("ls " + dir + "/")
	if err != nil {
		return nil, err
	}
	var slots []string
	for _, name := range strings.Fields(out) {
		if s, ok := strings.CutPrefix(name, "boot"); ok && (s == "_a" || s == "_b") {
			slots = append(slots, s)
		}
	}
	if len(slots) == 0 {
		return nil, fmt.Errorf("no boot partitions found")
	}
	return slots, nil
}
