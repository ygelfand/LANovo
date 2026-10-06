// Package boot brings the device up.
//
// It owns what lanovod is made of and in what order: which hardware is taken, which parts are
// supervised, and what is only needed once at start-up. The command line's job is to parse flags
// and call Run.
package boot

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/component"
	_ "github.com/ygelfand/LANovo/internal/component/all"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/parts"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/LANovo/internal/update"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

// Run brings everything up and stays until ctx is canceled.
//
// Hardware that cannot be taken is logged and left out: a device with no speaker still shows a
// screen, and nothing here is worth refusing to start over.
func Run(ctx context.Context) error {
	done, err := Only()
	if err != nil {
		// Loudly, and naming the holder. init discards stderr and cannot tell this from a crash,
		// so the log is the only place anyone will find out why the service is restarting.
		var taken *Taken
		if errors.As(err, &taken) {
			slog.Error("not starting: another lanovod already has the device",
				"holder", taken.Pid, "process", taken.Name, "exit", ExitTaken)
		}
		return err
	}
	defer done()

	keepCrashes(layout.CrashPath)

	slog.Info("lanovod starting", "version", layout.Version,
		"pid", os.Getpid(), "uid", os.Getuid(), "selinux", selinuxContext())

	// crypto/x509 reads the Android store only in a GOOS=android build.
	os.RemoveAll(layout.TempDir)
	if err := os.MkdirAll(layout.TempDir, 0o700); err != nil {
		slog.Error("making the temporary directory failed", "dir", layout.TempDir, "err", err)
	} else if err := os.Setenv("TMPDIR", layout.TempDir); err != nil {
		slog.Error("pointing temporary files at the data partition failed", "err", err)
	}
	if err := os.Setenv("SSL_CERT_DIR", layout.CertDirs); err != nil {
		slog.Error("pointing the verifier at the platform's roots failed", "err", err)
	}

	// What this process was told, put where everything else reads its settings from, so nothing has
	// to be handed a struct to find out what the device is called or where it listens. Before
	// anything else: the components read this the moment they are asked to restore.
	config.Started(config.Device{
		Name:  deviceName(),
		Addr:  listenAddr(),
		Model: deviceModel(),
	})
	if err := config.LoadError(); err != nil {
		slog.Error("reading the saved config failed, continuing with defaults", "err", err)
	}

	firmware.Get().RebootPending(parts.Ensure())
	update.Settle()

	// Which way up the device is, before anything is drawn: the panel opens at the rotation it is
	// usually stood in, and a logo that appears and then turns is the device telling you it was
	// not looking.
	sensors.Get().Orient()

	// The logo goes up before anything is restored, because it is the only thing the device can say
	// while the rest of it is still coming up. It takes a claim, so it can be asked for before the
	// panel is open, and holds it until every component that has something to say about coming up
	// says it is up.
	startSplash(ctx)

	// Everything the device remembers, put back in the order the components registered and before
	// anything is listening: how the device behaves is not Home Assistant's business.
	component.Default().Restore(config.Get())
	slog.Info("state restored")

	display.Get().Stranded(DrawStranded)

	group := service.New()
	component.Default().AddTo(group)

	slog.Info("resident")
	return group.Run(leaving(ctx))
}

// leavingWait is how long the restarting screen gets to reach the panel before the shutdown goes
// on without it. A frame is tens of milliseconds, so this is a bound on something going wrong.
//
// Nothing here survives init: ctl.restart and ctl.stop are KillProcessGroup(SIGKILL) on this
// release, with no signal to catch. Only a SIGTERM reaches this, which is what a stop from a shell
// sends and what the panel's restart sends itself.
var leavingWait = 2 * time.Second

// leaving is the group's context: canceled a moment after ctx, with the restarting screen painted
// in between.
//
// The painting happens here, while everything is still up, rather than on the way down. The render
// loop is running, the panel is open, and the driver stops drawing behind the frame, so the picture
// on the panel when the process exits is the one it chose. Doing it after the group has stopped
// would be too late: the display closes as it unwinds.
//
// A cold boot never shows this, because there was no process to paint it. A hard kill paints
// nothing and leaves the last frame up, which is a reasonable thing to leave up.
func leaving(ctx context.Context) context.Context {
	out, stop := context.WithCancel(context.WithoutCancel(ctx))
	wait := leavingWait

	safe.Go("leaving", func() {
		<-ctx.Done()
		defer stop()

		paint, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()

		if err := display.Get().Last(paint, DrawLeaving); err != nil {
			slog.Warn("the restarting screen did not go up", "err", err)
		}
	})
	return out
}

// selinuxContext is the domain lanovod is running in, which decides what it may touch. Logging it
// makes a permission failure obvious rather than mysterious.
func selinuxContext() string {
	b, err := os.ReadFile("/proc/self/attr/current")
	if err != nil {
		return "unknown"
	}
	return string(trimNUL(b))
}

func trimNUL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == 0 || b[len(b)-1] == '\n') {
		b = b[:len(b)-1]
	}
	return b
}

// deviceModel is the hardware as Lenovo's own partition names it.
func deviceModel() string {
	if model, err := prop.Local.Getprop(prop.Model); err == nil && model != "" {
		return model
	}
	return layout.Model
}

// deviceName is what Home Assistant shows. Chosen at install and written to the device.
func deviceName() string {
	b, err := os.ReadFile(layout.NamePath)
	if err == nil {
		if name := string(trimNUL(b)); name != "" {
			return name
		}
	}
	return layout.DefaultName
}

// listenAddr is where the API server listens.
func listenAddr() string { return layout.ListenAddr }
