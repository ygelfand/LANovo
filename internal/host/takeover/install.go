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

// Step is one thing an install did.
type Step struct {
	What string
	Note string
}

// Result is what an install did and what is left to do.
type Result struct {
	Steps []Step

	// Settles is true when something changed that only takes effect at boot: the service
	// definition and /default.prop, both of which init reads once. Replacing the binary is not one
	// of those — restarting the service is enough — so re-running an install does not ask for a
	// reboot it does not need.
	Settles bool
}

// Install puts lanovod and its init service on the device. lanovod writes the rest of its parts
// itself when it starts.
//
// binary is the lanovod to install. / is made writable for the duration and put back afterwards.
func Install(d *device.Device, binary []byte) (Result, error) {
	var res Result

	if len(binary) == 0 {
		return res, errors.New("no lanovod to install")
	}

	// Whether the service definition is about to change decides whether a reboot is wanted.
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

	// The service holds the binary open, so it cannot be replaced while running.
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
		res.Steps = append(res.Steps, Step{What: parts.InitRC.Name, Note: layout.InitRC + " unchanged"})
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

	// /system/bin and /system/etc are system_file, which is what init expects to exec and read.
	d.Shell("restorecon " + layout.Binary + " " + layout.InitRC)

	return res, nil
}

// Start runs the service, which is enough to get the panel back when only the binary changed.
func Start(d *device.Device) error {
	return prop.Start(d, layout.Service)
}

// WaitRunning blocks until lanovod is up after a reboot.
//
// Not sys.boot_completed: system_server sets that, and zygote is stopped, so it never arrives.
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
