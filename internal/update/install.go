package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/parts"
)

const downloadTimeout = 10 * time.Minute

var installing sync.Mutex

// Install replaces lanovod with the build the manifest offers for this architecture. Nothing goes
// near /system until the download is whole and its hash checked. The caller restarts into it.
func Install(ctx context.Context, m Manifest, progress func(float32)) error {
	if !installing.TryLock() {
		return fmt.Errorf("update: an install is already running")
	}
	defer installing.Unlock()

	if err := m.Valid(); err != nil {
		return err
	}
	b, err := m.For(arch)
	if err != nil {
		return err
	}

	defer os.Remove(layout.Incoming)
	if err := download(ctx, b, layout.Incoming, progress); err != nil {
		return err
	}
	if err := room(b.Size); err != nil {
		return err
	}
	return swap(layout.Incoming, m.Version)
}

func download(ctx context.Context, b Binary, to string, progress func(float32)) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: fetching %s: %w", b.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: fetching %s: %s", b.URL, resp.Status)
	}

	f, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()

	sum := sha256.New()
	written, err := io.Copy(
		io.MultiWriter(f, sum),
		&counter{from: io.LimitReader(resp.Body, b.Size+1), size: b.Size, report: progress},
	)
	if err != nil {
		return fmt.Errorf("update: downloading %s: %d of %d bytes: %w", b.URL, written, b.Size, err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if written != b.Size {
		return fmt.Errorf("update: %d bytes, offered as %d", written, b.Size)
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != b.SHA256 {
		return fmt.Errorf("update: hash %s, offered as %s", got, b.SHA256)
	}
	return nil
}

func room(need int64) error {
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err != nil {
		return nil
	}
	if free := int64(fs.Bavail) * int64(fs.Bsize); free < need {
		return fmt.Errorf("update: %d bytes free on /, need %d", free, need)
	}
	return nil
}

// swap moves the running binary aside, which a running executable allows where overwriting it does
// not, and writes the new one in its place.
func swap(staged, version string) error {
	restore, err := parts.Writable()
	if err != nil {
		return fmt.Errorf("update: remounting to install: %w", err)
	}
	defer func() {
		if err := restore(); err != nil {
			slog.Error("remounting read-only failed", "err", err)
		}
	}()

	if err := os.Rename(layout.Binary, layout.PrevBinary); err != nil {
		return fmt.Errorf("update: moving the running binary aside: %w", err)
	}
	data, err := os.ReadFile(staged)
	if err != nil {
		os.Rename(layout.PrevBinary, layout.Binary)
		return err
	}
	if err := writeBinary(data); err != nil {
		if back := os.Rename(layout.PrevBinary, layout.Binary); back != nil {
			slog.Error("could not put the previous binary back", "err", back)
		}
		return err
	}
	slog.Info("update installed, restarting into it", "version", version)
	return nil
}

func writeBinary(data []byte) error {
	f, err := os.OpenFile(layout.Binary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("update: writing %s: %w", layout.Binary, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return parts.CopyLabel(layout.PrevBinary, layout.Binary)
}

// Settle deletes the binary the last update replaced, once the new one is running.
func Settle() {
	if _, err := os.Stat(layout.PrevBinary); err != nil {
		return
	}
	restore, err := parts.Writable()
	if err != nil {
		slog.Error("remounting to clear the previous binary failed", "err", err)
		return
	}
	defer func() {
		if err := restore(); err != nil {
			slog.Error("remounting read-only failed", "err", err)
		}
	}()
	if err := os.Remove(layout.PrevBinary); err != nil {
		slog.Error("removing the previous binary failed", "err", err)
		return
	}
	slog.Info("previous binary removed", "path", layout.PrevBinary)
}

type counter struct {
	from   io.Reader
	size   int64
	report func(float32)

	read int64
	last float32
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.from.Read(p)
	c.read += int64(n)
	if c.report == nil || c.size <= 0 {
		return n, err
	}
	if at := float32(c.read) / float32(c.size); at-c.last >= 0.01 {
		c.last = at
		c.report(at)
	}
	return n, err
}
