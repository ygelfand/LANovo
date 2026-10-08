package takeover

import "testing"

// What `stat -f -c "%f %S" /` prints on the device.
func TestReadingFreeSpace(t *testing.T) {
	got, err := parseFree("9389 4096\n")
	if err != nil {
		t.Fatalf("parseFree: %v", err)
	}
	if want := int64(9389 * 4096); got != want {
		t.Errorf("free space read as %d, want %d", got, want)
	}
}

func TestUnreadableFreeSpaceIsAnError(t *testing.T) {
	for _, in := range []string{
		"",
		"9389",
		"stat: unknown option -- f",
		"lots 4096",
		"9389 nope",
		"9389 0",
		"-1 4096",
	} {
		if got, err := parseFree(in); err == nil {
			t.Errorf("%q was read as %d bytes free", in, got)
		}
	}
}

func TestReadingAFileSize(t *testing.T) {
	got, err := parseSize(" 21168290\n")
	if err != nil {
		t.Fatalf("parseSize: %v", err)
	}
	if got != 21168290 {
		t.Errorf("size read as %d", got)
	}
}

func TestUnreadableFileSizeIsAnError(t *testing.T) {
	if _, err := parseSize("stat: '/system/bin/lanovod': No such file or directory"); err == nil {
		t.Error("an error message was read as a size")
	}
}

func TestMegabytesReadTheWayAnInstallerWouldWriteThem(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0.0 MB"},
		{21168290, "20.2 MB"},
		{RoomMargin, "4.0 MB"},
	} {
		if got := megabytes(tc.in); got != tc.want {
			t.Errorf("%d bytes read as %q, want %q", tc.in, got, tc.want)
		}
	}
}
