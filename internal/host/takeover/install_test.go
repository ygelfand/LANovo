package takeover

import (
	"errors"
	"os"
	"strings"
	"testing"
)

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

func TestInitImportWhitespaceAndComments(t *testing.T) {
	const path = "/system/etc/init/lanovod.rc"
	commented := []byte("# import " + path + "\non\tboot\n    start vendor")
	got := appendInitImport(commented, path)
	if want := "# import " + path + "\nimport " + path + "\non\tboot\n    start vendor"; string(got) != want {
		t.Fatalf("appended init = %q, want %q", got, want)
	}
	existing := []byte("import\t\"" + path + "\"\n")
	if got := appendInitImport(existing, path); string(got) != string(existing) {
		t.Fatalf("modified existing import: %q", got)
	}
}

type initFiles struct {
	files     map[string][]byte
	corrupt   bool
	committed bool
}

func (d *initFiles) ReadFile(path string) ([]byte, error) {
	data, ok := d.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (d *initFiles) WriteFile(path string, data []byte, _ os.FileMode) error {
	if d.corrupt {
		d.files[path] = data[:len(data)/2]
		return nil
	}
	d.files[path] = data
	return nil
}

func (d *initFiles) Shell(cmd string) (string, error) {
	fields := strings.Fields(cmd)
	switch fields[0] {
	case "mktemp":
		return strings.TrimSuffix(fields[1], "XXXXXXXX") + "abcd1234\n", nil
	case "ln":
		d.files[fields[2]] = d.files[fields[1]]
	case "rm":
		delete(d.files, fields[2])
	case "chown":
		_, args, found := strings.Cut(cmd, " && mv -f ")
		if !found {
			return "", errors.New("expected an atomic rename")
		}
		move := strings.Fields(args)
		d.files[move[1]] = d.files[move[0]]
		delete(d.files, move[0])
		d.committed = true
	default:
		return "", errors.New("unexpected shell command")
	}
	return "", nil
}

func (d *initFiles) ShellCode(cmd string) (string, int, error) {
	path, ok := strings.CutPrefix(cmd, "test -e ")
	if !ok {
		return "", 2, errors.New("unexpected shell command")
	}
	if _, ok := d.files[path]; !ok {
		return "", 1, nil
	}
	return "", 0, nil
}

func TestOEMInitStagingCanRetryAfterTruncatedWrite(t *testing.T) {
	const path = "/init.msm8x53.rc"
	original := []byte("on boot\n    start vendor\n")
	updated := appendInitImport(original, "/system/etc/init/lanovod.rc")
	d := &initFiles{files: map[string][]byte{path: original}, corrupt: true}
	backup, err := backupOEMInit(d, path, original)
	if err != nil {
		t.Fatal(err)
	}
	if err := stageInstallFile(d, path, updated, 0644); err == nil || d.committed {
		t.Fatal("truncated staged file was accepted")
	}
	if len(d.files) != 2 || string(d.files[path]) != string(original) {
		t.Fatal("interrupted write left the OEM file changed or a staged file behind")
	}
	if _, err := backupOEMInit(d, path, original); err != nil {
		t.Fatalf("retry should reuse backup: %v", err)
	}
	d.corrupt = false
	if err := stageInstallFile(d, path, updated, 0644); err != nil {
		t.Fatal(err)
	}
	if string(d.files[path]) != string(updated) || string(d.files[backup]) != string(original) {
		t.Fatal("retry should publish the complete file and keep the original")
	}
}

func TestOEMInitRejectsAnUnrelatedBackup(t *testing.T) {
	const path = "/init.msm8x53.rc"
	d := &initFiles{files: map[string][]byte{path: []byte("OEM"), path + ".lanovo.bak": []byte("other")}}
	if _, err := backupOEMInit(d, path, d.files[path]); err == nil {
		t.Fatal("accepted a backup that does not match the OEM file")
	}
}
