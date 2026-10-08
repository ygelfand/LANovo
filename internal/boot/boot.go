package boot

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

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
)

func Run(ctx context.Context) error {
	done, err := Only()
	if err != nil {
		// init discards stderr.
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
	if err := os.RemoveAll(layout.TempDir); err != nil {
		slog.Error("clearing the temporary directory failed", "dir", layout.TempDir, "err", err)
	}
	if err := os.MkdirAll(layout.TempDir, 0o700); err != nil {
		slog.Error("making the temporary directory failed", "dir", layout.TempDir, "err", err)
	} else if err := os.Setenv("TMPDIR", layout.TempDir); err != nil {
		slog.Error("pointing temporary files at the data partition failed", "err", err)
	}
	if err := os.Setenv("SSL_CERT_DIR", layout.CertDirs); err != nil {
		slog.Error("pointing the verifier at the platform's roots failed", "err", err)
	}

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

	sensors.Get().Orient()

	startSplash(ctx)

	component.Default().Restore(config.Get())
	slog.Info("state restored")

	display.Get().Stranded(DrawStranded)

	group := service.New()
	component.Default().AddTo(group)

	slog.Info("resident")
	return group.Run(leaving(ctx))
}

// init's ctl.restart and ctl.stop are KillProcessGroup(SIGKILL) on this release.
var leavingWait = 2 * time.Second

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

func deviceModel() string {
	if model, err := prop.Local.Getprop(prop.Model); err == nil && model != "" {
		return model
	}
	return layout.Model
}

func deviceName() string {
	b, err := os.ReadFile(layout.NamePath)
	if err == nil {
		if name := string(trimNUL(b)); name != "" {
			return name
		}
	}
	return layout.DefaultName
}

func listenAddr() string { return layout.ListenAddr }
