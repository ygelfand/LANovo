package takeover

import "testing"

func TestHasInitImportMatchesExactDirective(t *testing.T) {
	rc := []byte("on early-init\n    mkdir /dev/foo 0755 root root\n\nimport /system/etc/init/lanovod.rc\n")
	if !hasInitImport(rc, "/system/etc/init/lanovod.rc") {
		t.Fatal("expected exact import directive to be found")
	}
	if hasInitImport(rc, "/system/etc/init/other.rc") {
		t.Fatal("unexpected import directive match")
	}
}

func TestAppendInitImportPreservesOEMRCAndAddsOneDirective(t *testing.T) {
	stock := []byte("# OEM init config\r\non boot\r\n    setprop vendor.ready 1\r\n\r\n")
	got := appendInitImport(stock, "/system/etc/init/lanovod.rc")
	want := "# OEM init config\r\nimport /system/etc/init/lanovod.rc\r\non boot\r\n    setprop vendor.ready 1\r\n\r\n"
	if string(got) != want {
		t.Fatalf("unexpected merged init config:\n%q\nwant:\n%q", got, want)
	}
	got = appendInitImport(got, "/system/etc/init/lanovod.rc")
	if string(got) != want {
		t.Fatal("adding the same import twice changed the init config")
	}
}
