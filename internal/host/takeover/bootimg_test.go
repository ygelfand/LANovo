package takeover

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func bootPage(pageSize uint32, cmdline string) []byte {
	page := make([]byte, max(int(pageSize), headerRead))

	copy(page, bootMagic)
	binary.LittleEndian.PutUint32(page[36:40], pageSize)
	copy(page[cmdlineOffset:], cmdline)
	return page
}

const realCmdline = "core_ctl_disable_cpumask=0-7 kpti=0 console=ttyMSM0,115200,n8 earlyprintk " +
	"androidboot.hardware=msm8x53 firmware_class.path=/oem/firmware buildvariant=userdebug " +
	"androidboot.selinux=permissive androidboot.bootdevice=7824900.sdhci " +
	"androidboot.verifiedbootstate=orange root=PARTUUID=c5369f5b-41c7-40f6-a816-833595d07a58"

func TestHeader(t *testing.T) {
	page := bootPage(2048, "console=ttyMSM0 root=/dev/foo")

	size, cmdline, err := header(page)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	if size != 2048 {
		t.Errorf("page size = %d, want 2048", size)
	}
	if cmdline != "console=ttyMSM0 root=/dev/foo" {
		t.Errorf("cmdline = %q", cmdline)
	}
}

func TestHeaderRefusesWhatIsNotABootImage(t *testing.T) {
	tests := []struct {
		name string
		page []byte
	}{
		{"empty", nil},
		{"too short to hold a cmdline", make([]byte, 100)},
		{"no magic", make([]byte, headerRead)},
		{"a page size of zero", func() []byte {
			p := bootPage(2048, "x")
			binary.LittleEndian.PutUint32(p[36:40], 0)
			return p
		}()},
		{"an implausible page size", func() []byte {
			p := bootPage(2048, "x")
			binary.LittleEndian.PutUint32(p[36:40], 1<<20)
			return p
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := header(tt.page); err == nil {
				t.Error("accepted something that is not a boot image")
			}
		})
	}
}

func TestHeaderTrimsThePadding(t *testing.T) {
	page := bootPage(2048, "console=ttyMSM0")

	_, cmdline, err := header(page)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	if strings.ContainsRune(cmdline, 0) {
		t.Errorf("cmdline carries padding: %q", cmdline)
	}
}

func TestPermissiveAppends(t *testing.T) {
	got, err := permissive("console=ttyMSM0")
	if err != nil {
		t.Fatalf("permissive: %v", err)
	}
	if got != "console=ttyMSM0 "+PermissiveArg {
		t.Errorf("got %q", got)
	}
}

func TestPermissiveRefusesWhatWillNotFit(t *testing.T) {
	long := strings.Repeat("a", cmdlineSize-len(PermissiveArg))

	if _, err := permissive(long); err == nil {
		t.Error("accepted a cmdline that would not fit")
	}
}

func TestPermissiveLeavesRoomForTheTerminator(t *testing.T) {
	base := strings.Repeat("a", cmdlineSize-len(PermissiveArg)-2)

	got, err := permissive(base)
	if err != nil {
		t.Fatalf("permissive: %v", err)
	}
	if len(got) >= cmdlineSize {
		t.Errorf("the result is %d bytes, which fills a %d byte field", len(got), cmdlineSize)
	}
}

func TestWriteCmdlineTouchesNothingElse(t *testing.T) {
	page := bootPage(2048, realCmdline)

	for i := cmdlineOffset + cmdlineSize; i < len(page); i++ {
		page[i] = byte(i)
	}
	before := bytes.Clone(page)

	want, err := permissive(realCmdline)
	if err != nil {
		t.Fatalf("permissive: %v", err)
	}
	if err := writeCmdline(page, want); err != nil {
		t.Fatalf("writeCmdline: %v", err)
	}

	if !bytes.Equal(page[:cmdlineOffset], before[:cmdlineOffset]) {
		t.Error("the header before the cmdline changed")
	}

	after := cmdlineOffset + cmdlineSize
	if !bytes.Equal(page[after:], before[after:]) {
		t.Error("the page after the cmdline changed")
	}
}

func TestWriteCmdlineReplacesRatherThanAppends(t *testing.T) {
	page := bootPage(2048, "console=ttyMSM0 something=long-enough-to-leave-a-tail")

	if err := writeCmdline(page, "short=1"); err != nil {
		t.Fatalf("writeCmdline: %v", err)
	}

	_, cmdline, err := header(page)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	if cmdline != "short=1" {
		t.Errorf("cmdline = %q, want the old one gone", cmdline)
	}
}

func TestPatchRoundTrip(t *testing.T) {
	page := bootPage(2048, realCmdline)

	_, cmdline, err := header(page)
	if err != nil {
		t.Fatalf("header: %v", err)
	}

	want, err := permissive(cmdline)
	if err != nil {
		t.Fatalf("permissive: %v", err)
	}
	if err := writeCmdline(page, want); err != nil {
		t.Fatalf("writeCmdline: %v", err)
	}

	_, got, err := header(page)
	if err != nil {
		t.Fatalf("header after writing: %v", err)
	}
	if !strings.Contains(got, PermissiveArg) {
		t.Errorf("the argument did not take: %q", got)
	}
	if !strings.HasPrefix(got, "core_ctl_disable_cpumask=0-7") {
		t.Errorf("the original cmdline was lost: %q", got)
	}
	if !strings.Contains(got, "root=PARTUUID=c5369f5b-41c7-40f6-a816-833595d07a58") {
		t.Errorf("the root argument was lost: %q", got)
	}
}

func TestWriteCmdlineRefusesAShortPage(t *testing.T) {
	if err := writeCmdline(make([]byte, 100), "x=1"); err == nil {
		t.Error("accepted a page too short to hold a cmdline")
	}
}

func TestDecodeBase64(t *testing.T) {
	const encoded = "QU5EUk9JRCE=\r\n"

	got, err := decodeBase64(encoded)
	if err != nil {
		t.Fatalf("decodeBase64: %v", err)
	}
	if string(got) != "ANDROID!" {
		t.Errorf("decoded %q", got)
	}
}

func TestDecodeBase64IgnoresLineEndings(t *testing.T) {
	const wrapped = "QU5E\r\nUk9J\nRCE=\n"

	got, err := decodeBase64(wrapped)
	if err != nil {
		t.Fatalf("decodeBase64: %v", err)
	}
	if string(got) != "ANDROID!" {
		t.Errorf("decoded %q", got)
	}
}

func TestChatterIsCaughtByTheMagicRatherThanTheDecoder(t *testing.T) {
	page := bootPage(2048, realCmdline)

	corrupted := append([]byte("1+0 records in"), page...)

	if _, _, err := header(corrupted); err == nil {
		t.Error("a corrupted read was accepted as a boot image")
	}
}

func TestWriteCmdlineRefusesAPageThatIsNotABootImage(t *testing.T) {
	page := bootPage(2048, realCmdline)
	copy(page, "NOTBOOT!")

	if err := writeCmdline(page, "console=ttyMSM0"); err == nil {
		t.Error("a page with the wrong magic was written to")
	}
}

func TestWriteCmdlineRefusesAPageOfZeroes(t *testing.T) {
	if err := writeCmdline(make([]byte, 2048), "console=ttyMSM0"); err == nil {
		t.Error("a page of zeroes was written to")
	}
}

func TestWriteCmdlineAcceptsARealPage(t *testing.T) {
	page := bootPage(2048, realCmdline)

	if err := writeCmdline(page, "console=ttyMSM0 androidboot.selinux=permissive"); err != nil {
		t.Fatalf("writeCmdline: %v", err)
	}

	_, cmdline, err := header(page)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	if cmdline != "console=ttyMSM0 androidboot.selinux=permissive" {
		t.Errorf("cmdline = %q", cmdline)
	}
}
