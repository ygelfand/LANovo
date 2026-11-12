package takeover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

func Install(d *adb.Device, binary []byte, b board.Board) (Result, error) {
	var res Result

	if len(binary) == 0 {
		return res, errors.New("no lanovod to install")
	}
	paths := b.Paths()
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
		initImport, err = d.ReadFile(initImportPath)
		if err != nil {
			return res, fmt.Errorf("reading %s: %w", initImportPath, err)
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
		if err := restore(); err != nil {
			res.Steps = append(res.Steps, Step{What: "remount read-only", Note: err.Error()})
		}
	}()

	// The running service holds the binary open.
	if err := prop.Stop(d, layout.Service); err != nil {
		return res, err
	}
	if b.Name == "sd-x501f" {
		if _, err := d.Shell("mkdir -p /data/misc/lanovo/bin"); err != nil {
			return res, fmt.Errorf("creating LANovo data directory: %w", err)
		}
	}

	if err := d.WriteFile(paths.Binary, binary, parts.Lanovod.Mode); err != nil {
		return res, fmt.Errorf("installing %s: %w", paths.Binary, err)
	}
	res.Steps = append(res.Steps, Step{What: parts.Lanovod.Name, Note: paths.Binary})
	if b.Name == "sd-x501f" {
		wrapper := []byte("#!/system/bin/sh\nexec /system/bin/lanovod run --android-pcm /data/misc/lanovo/bin/x501f-hal-tone\n")
		if err := d.WriteFile("/data/misc/lanovo/bin/launch-lanovod.sh", wrapper, 0755); err != nil {
			return res, fmt.Errorf("installing LANovo launch wrapper: %w", err)
		}
		waiter := []byte("#!/system/bin/sh\nwhile :; do\n  read uptime rest < /proc/uptime || exit 1\n  uptime=${uptime%%.*}\n  case \"$uptime\" in *[!0-9]*|\x27\x27) exit 1 ;; esac\n  [ \"$uptime\" -ge 20 ] && break\n  sleep 1\ndone\nsetprop lanovo.takeover 1\n")
		if err := d.WriteFile("/data/misc/lanovo/bin/wait-lanovo.sh", waiter, 0755); err != nil {
			return res, fmt.Errorf("installing LANovo boot waiter: %w", err)
		}
		if len(parts.X501FAudio()) == 0 {
			return res, errors.New("missing SD-X501F AudioTrack helper")
		}
		if err := d.WriteFile("/data/misc/lanovo/bin/x501f-hal-tone", parts.X501FAudio(), 0755); err != nil {
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
			if err := d.WriteFile(part.Path, part.Data(), part.Mode); err != nil {
				return res, fmt.Errorf("installing %s: %w", part.Path, err)
			}
			if _, err := d.Shell("restorecon " + part.Path); err != nil {
				return res, fmt.Errorf("labelling %s: %w", part.Path, err)
			}
			res.Steps = append(res.Steps, Step{What: part.Name, Note: part.Path})
		}
	}

	if res.Settles {
		if err := d.WriteFile(paths.InitRC, rc, parts.InitRC.Mode); err != nil {
			return res, fmt.Errorf("installing %s: %w", paths.InitRC, err)
		}
		res.Steps = append(res.Steps, Step{What: parts.InitRC.Name, Note: paths.InitRC})
	} else {
		res.Steps = append(
			res.Steps,
			Step{What: parts.InitRC.Name, Note: paths.InitRC + " unchanged"},
		)
	}
	if initImportPath != "" && !hasInitImport(initImport, paths.InitRC) {
		backupPath := initImportPath + ".lanovo.bak"
		if _, err := d.Shell("test -e " + backupPath); err != nil {
			if err := d.WriteFile(backupPath, initImport, parts.InitRC.Mode); err != nil {
				return res, fmt.Errorf("backing up %s to %s: %w", initImportPath, backupPath, err)
			}
			res.Steps = append(res.Steps, Step{What: "init backup", Note: backupPath})
		}
		updated := appendInitImport(initImport, paths.InitRC)
		if err := d.WriteFile(initImportPath, updated, parts.InitRC.Mode); err != nil {
			return res, fmt.Errorf("adding LANovo import to %s: %w", initImportPath, err)
		}
		res.Steps = append(res.Steps, Step{What: "init import", Note: initImportPath})
	}

	secured, err := MakeInsecure(d, b.Name)
	if err != nil {
		return res, err
	}
	if err := nameBoard(d, b); err != nil {
		return res, err
	}
	res.Settles = res.Settles || secured

	note := Insecure + " already set"
	if secured {
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

	return res, nil
}

func hasInitImport(rc []byte, path string) bool {
	want := "import " + path
	for _, line := range strings.Split(string(rc), "\n") {
		if strings.TrimSpace(line) == want {
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
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("import ")) ||
			bytes.HasPrefix(trimmed, []byte("on ")) ||
			bytes.HasPrefix(trimmed, []byte("service ")) {
			break
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
