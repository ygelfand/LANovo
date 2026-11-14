package takeover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/parts"
)

type Step struct {
	What string
	Note string
}

type Result struct {
	Steps []Step

	Settles bool
}

func Install(d *adb.Device, binary []byte, b board.Board) (res Result, err error) {
	if len(binary) == 0 {
		return res, errors.New("no lanovod to install")
	}
	paths := b.Paths()
	if paths.X501F && !parts.HasX501FAudio() {
		return res, errors.New("missing SD-X501F audio helpers; build with BOARD=sd-x501f")
	}
	if !paths.OnSystem() {
		return installData(d, binary, b, paths)
	}

	rc, err := parts.RC(paths)
	if err != nil {
		return res, err
	}
	existing, _ := d.ReadFile(paths.InitRC)
	res.Settles = !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(rc))
	var initImportPath string
	var initImport []byte
	if b.Name == "sd-x501f" {
		// Android 8.1 on the SD-X501F does not import /system/etc/init/*.rc.
		// Its init.msm8x53.rc lives at the system-as-root mount point, so add
		// one import there while keeping the OEM file and LANovo rc separate.
		initImportPath = "/init.msm8x53.rc"
		if _, err := d.Shell("test -s " + initImportPath); err != nil {
			return res, fmt.Errorf("checking %s: %w", initImportPath, err)
		}
		initImport, err = d.ReadFile(initImportPath)
		if err != nil {
			return res, fmt.Errorf("reading %s: %w", initImportPath, err)
		}
		if len(bytes.TrimSpace(initImport)) == 0 {
			return res, fmt.Errorf("%s is empty; restore the OEM init file first", initImportPath)
		}
		if !hasInitImport(initImport, paths.InitRC) {
			res.Settles = true
		}
	}

	restore, err := Writable(d)
	if err != nil {
		return res, err
	}
	defer func() {
		if restoreErr := restore(); restoreErr != nil {
			res.Steps = append(res.Steps, Step{What: "remount read-only", Note: restoreErr.Error()})
			if paths.X501F {
				err = errors.Join(err, restoreErr)
			}
		}
	}()

	addImport := initImportPath != "" && !hasInitImport(initImport, paths.InitRC)
	if addImport {
		backup, err := backupOEMInit(d, initImportPath, initImport)
		if err != nil {
			return res, err
		}
		res.Steps = append(res.Steps, Step{What: "init backup", Note: backup})
	}

	write := d.WriteFile
	if paths.X501F {
		write = func(path string, data []byte, mode os.FileMode) error {
			return stageInstallFile(d, path, data, mode)
		}
	}

	// Stop the running instance before upgrading files used by its helpers.
	if err := prop.Stop(d, layout.Service); err != nil {
		return res, err
	}
	if b.Name == "sd-x501f" {
		if _, err := d.Shell("mkdir -p /data/misc/lanovo/bin"); err != nil {
			return res, fmt.Errorf("creating LANovo data directory: %w", err)
		}
	}

	if !paths.X501F {
		if err := write(paths.Binary, binary, parts.Lanovod.Mode); err != nil {
			return res, fmt.Errorf("installing %s: %w", paths.Binary, err)
		}
		res.Steps = append(res.Steps, Step{What: parts.Lanovod.Name, Note: paths.Binary})
	}
	if b.Name == "sd-x501f" {
		wrapper := []byte("#!/system/bin/sh\n/data/misc/lanovo/bin/x501f-audio-policy 100 || log -t lanovo 'failed to set Android speaker volume'\nexec /system/bin/lanovod run --android-pcm /data/misc/lanovo/bin/x501f-hal-tone\n")
		if err := write("/data/misc/lanovo/bin/launch-lanovod.sh", wrapper, 0755); err != nil {
			return res, fmt.Errorf("installing LANovo launch wrapper: %w", err)
		}
		waiter := []byte("#!/system/bin/sh\nwhile :; do\n  read uptime rest < /proc/uptime || exit 1\n  uptime=${uptime%%.*}\n  case \"$uptime\" in *[!0-9]*|\x27\x27) exit 1 ;; esac\n  [ \"$uptime\" -ge 20 ] && break\n  sleep 1\ndone\nsetprop lanovo.takeover 1\n")
		if err := write("/data/misc/lanovo/bin/wait-lanovo.sh", waiter, 0755); err != nil {
			return res, fmt.Errorf("installing LANovo boot waiter: %w", err)
		}
		if err := write("/data/misc/lanovo/bin/x501f-audio-policy", parts.X501FAudioPolicy(), 0755); err != nil {
			return res, fmt.Errorf("installing SD-X501F audio policy helper: %w", err)
		}
		if err := write("/data/misc/lanovo/bin/x501f-hal-tone", parts.X501FAudio(), 0755); err != nil {
			return res, fmt.Errorf("installing SD-X501F AudioTrack helper: %w", err)
		}
		carried, err := parts.Carried(paths)
		if err != nil {
			return res, err
		}
		for _, part := range carried {
			if part.Boot || len(part.Data()) == 0 {
				continue
			}
			if err := write(part.Path, part.Data(), part.Mode); err != nil {
				return res, fmt.Errorf("installing %s: %w", part.Path, err)
			}
			res.Steps = append(res.Steps, Step{What: part.Name, Note: part.Path})
		}
	}

	if res.Settles {
		if err := write(paths.InitRC, rc, parts.InitRC.Mode); err != nil {
			return res, fmt.Errorf("installing %s: %w", paths.InitRC, err)
		}
		res.Steps = append(res.Steps, Step{What: parts.InitRC.Name, Note: paths.InitRC})
	} else {
		res.Steps = append(
			res.Steps,
			Step{What: parts.InitRC.Name, Note: paths.InitRC + " unchanged"},
		)
	}
	if paths.X501F {
		if err := write(paths.Binary, binary, parts.Lanovod.Mode); err != nil {
			return res, fmt.Errorf("installing %s: %w", paths.Binary, err)
		}
		res.Steps = append(res.Steps, Step{What: parts.Lanovod.Name, Note: paths.Binary})
	}
	reboot, err := configureDefaultProp(d, b)
	if err != nil {
		return res, err
	}
	if err := nameBoard(d, b); err != nil {
		return res, err
	}
	res.Settles = res.Settles || reboot

	note := Insecure + " already set"
	if paths.X501F {
		note = prop.Board + "=" + b.Name
	} else if reboot {
		note = Insecure + " — adbd starts as root from the next boot"
	}
	res.Steps = append(res.Steps, Step{What: "default.prop", Note: note})

	// /system/bin and /system/etc are labelled system_file.
	restorecon := paths.Binary + " " + paths.InitRC
	if initImportPath != "" {
		restorecon += " " + initImportPath
	}
	if _, err := d.Shell("restorecon " + restorecon); err != nil {
		res.Steps = append(res.Steps, Step{What: "restorecon", Note: err.Error()})
	}

	if addImport {
		updated := appendInitImport(initImport, paths.InitRC)
		if err := stageInstallFile(d, initImportPath, updated, parts.InitRC.Mode); err != nil {
			return res, fmt.Errorf("adding LANovo import to %s: %w", initImportPath, err)
		}
		res.Steps = append(res.Steps, Step{What: "init import", Note: initImportPath})
	}

	return res, nil
}

type installFiles interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, os.FileMode) error
	Shell(string) (string, error)
	ShellCode(string) (string, int, error)
}

func backupOEMInit(d installFiles, path string, original []byte) (string, error) {
	backup := path + ".lanovo.bak"
	// A hard link preserves the OEM inode and metadata. A retry may reuse it
	// only when it still matches the live file before inserting the import.
	_, code, err := d.ShellCode("test -e " + backup)
	if err != nil {
		return "", err
	}
	switch code {
	case 0:
	case 1:
		if _, err := d.Shell("ln " + path + " " + backup + " && sync"); err != nil {
			return "", fmt.Errorf("backing up %s: %w", path, err)
		}
	default:
		return "", fmt.Errorf("checking %s: exit status %d", backup, code)
	}
	saved, err := d.ReadFile(backup)
	if err != nil {
		return "", fmt.Errorf("reading init backup: %w", err)
	}
	if !bytes.Equal(saved, original) {
		return "", fmt.Errorf("init backup %s differs from the current OEM file", backup)
	}
	return backup, nil
}

// Stage on the same filesystem so an interrupted transfer leaves the live file intact.
func stageInstallFile(d installFiles, path string, data []byte, mode os.FileMode) error {
	out, err := d.Shell("mktemp " + path + ".lanovo.XXXXXXXX")
	if err != nil {
		return err
	}
	temp := strings.TrimSpace(out)
	suffix, ok := strings.CutPrefix(temp, path+".lanovo.")
	if !ok || len(suffix) != 8 || strings.ContainsFunc(suffix, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		return fmt.Errorf("unexpected init staging path %q", temp)
	}
	defer func() { _, _ = d.Shell("rm -f " + temp) }()
	if err := d.WriteFile(temp, data, mode); err != nil {
		return err
	}
	got, err := d.ReadFile(temp)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, data) {
		return errors.New("staged init file did not pass readback")
	}
	_, err = d.Shell("chown 0:0 " + temp + " && restorecon " + temp +
		" && sync && mv -f " + temp + " " + path + " && restorecon " + path + " && sync")
	return err
}

func hasInitImport(rc []byte, path string) bool {
	for _, line := range strings.Split(string(rc), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "import" && strings.Trim(fields[1], "\"") == path {
			return true
		}
	}
	return false
}

func appendInitImport(rc []byte, path string) []byte {
	if hasInitImport(rc, path) {
		return append([]byte(nil), rc...)
	}
	newline := []byte("\n")
	if bytes.Contains(rc, []byte("\r\n")) {
		newline = []byte("\r\n")
	}
	lines := bytes.SplitAfter(rc, newline)
	var offset int
header:
	for _, line := range lines {
		fields := bytes.Fields(line)
		if len(fields) > 0 {
			switch string(fields[0]) {
			case "import", "on", "service":
				break header
			}
		}
		offset += len(line)
	}
	importLine := append([]byte("import "+path), newline...)
	updated := make([]byte, 0, len(rc)+len(importLine)+len(newline))
	updated = append(updated, rc[:offset]...)
	if offset > 0 && !bytes.HasSuffix(rc[:offset], newline) {
		updated = append(updated, newline...)
	}
	updated = append(updated, importLine...)
	updated = append(updated, rc[offset:]...)
	return updated
}

// The boot image carries root, the permissive policy and the service definition.
func installData(d *adb.Device, binary []byte, b board.Board, paths layout.Paths) (Result, error) {
	var res Result
	if err := prop.Stop(d, layout.Service); err != nil {
		return res, err
	}
	if err := d.WriteFile(paths.Binary, binary, parts.Lanovod.Mode); err != nil {
		return res, fmt.Errorf("installing %s: %w", paths.Binary, err)
	}
	res.Steps = append(res.Steps, Step{What: parts.Lanovod.Name, Note: paths.Binary})
	res.Steps = append(
		res.Steps,
		Step{What: parts.InitRC.Name, Note: paths.InitRC + " in the boot image"},
	)
	return res, nameBoard(d, b)
}

func nameBoard(d *adb.Device, b board.Board) error {
	if now, err := d.Getprop(prop.Board); err == nil && now == "" {
		return d.Setprop(prop.Board, b.Name)
	}
	return nil
}

func Start(d *adb.Device) error {
	return prop.Start(d, layout.Service)
}

// system_server sets sys.boot_completed, and zygote is stopped.
func WaitRunning(ctx context.Context, d *adb.Device) error {
	return poll(ctx, layout.Service+" did not start within two minutes", func() bool {
		return prop.Running(d, layout.Service)
	})
}

func WaitParts(ctx context.Context, d *adb.Device, paths layout.Paths) error {
	return poll(ctx, layout.Service+" ctl did not answer within two minutes", func() bool {
		_, err := d.Shell(paths.Binary + " ctl device version")
		return err == nil
	})
}

// init disables a service whose program was missing when it tried to start it.
var placedServices = []string{layout.SurfaceService, layout.CameraService}

func Down(d *adb.Device) []string {
	var down []string
	for _, s := range placedServices {
		if !prop.Running(d, s) {
			down = append(down, s)
		}
	}
	return down
}

func poll(ctx context.Context, timeout string, ready func() bool) error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if ready() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New(timeout)
}
