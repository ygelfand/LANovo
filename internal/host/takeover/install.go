package takeover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	if err := d.WriteFile(paths.Binary, binary, parts.Lanovod.Mode); err != nil {
		return res, fmt.Errorf("installing %s: %w", paths.Binary, err)
	}
	res.Steps = append(res.Steps, Step{What: parts.Lanovod.Name, Note: paths.Binary})

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
	if _, err := d.Shell("restorecon " + paths.Binary + " " + paths.InitRC); err != nil {
		res.Steps = append(res.Steps, Step{What: "restorecon", Note: err.Error()})
	}

	return res, nil
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
