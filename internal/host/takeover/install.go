package takeover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/host/device"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/parts"
)

var InitRC = parts.InitRC.Data()

type Step struct {
	What string
	Note string
}

type Result struct {
	Steps []Step

	Settles bool
}

func Install(d *device.Device, binary []byte) (Result, error) {
	var res Result

	if len(binary) == 0 {
		return res, errors.New("no lanovod to install")
	}

	existing, _ := d.ReadFile(layout.InitRC)
	res.Settles = !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(InitRC))

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

	if err := d.WriteFile(layout.Binary, binary, parts.Lanovod.Mode); err != nil {
		return res, fmt.Errorf("installing %s: %w", layout.Binary, err)
	}
	res.Steps = append(res.Steps, Step{What: parts.Lanovod.Name, Note: layout.Binary})

	if res.Settles {
		if err := d.WriteFile(layout.InitRC, InitRC, parts.InitRC.Mode); err != nil {
			return res, fmt.Errorf("installing %s: %w", layout.InitRC, err)
		}
		res.Steps = append(res.Steps, Step{What: parts.InitRC.Name, Note: layout.InitRC})
	} else {
		res.Steps = append(
			res.Steps,
			Step{What: parts.InitRC.Name, Note: layout.InitRC + " unchanged"},
		)
	}

	secured, err := MakeInsecure(d)
	if err != nil {
		return res, err
	}
	res.Settles = res.Settles || secured

	note := Insecure + " already set"
	if secured {
		note = Insecure + " — adbd starts as root from the next boot"
	}
	res.Steps = append(res.Steps, Step{What: "default.prop", Note: note})

	// /system/bin and /system/etc are labelled system_file.
	if _, err := d.Shell("restorecon " + layout.Binary + " " + layout.InitRC); err != nil {
		return res, fmt.Errorf("relabelling %s: %w", layout.Binary, err)
	}

	return res, nil
}

func Start(d *device.Device) error {
	return prop.Start(d, layout.Service)
}

// system_server sets sys.boot_completed, and zygote is stopped.
func WaitRunning(ctx context.Context, d *device.Device) error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if prop.Running(d, layout.Service) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("%s did not start within two minutes", layout.Service)
}
