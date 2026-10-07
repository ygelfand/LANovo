package drm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/dhcp"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

func init() {
	component.Register(component.Device, Get, component.Order(61))
}

const (
	checkSession = 1 << 31
	provisionFor = 30 * time.Second
)

var probe = []byte{
	0x00, 0x00, 0x00, 0x3e, 0x70, 0x73, 0x73, 0x68, 0x00, 0x00, 0x00, 0x00, 0xed, 0xef, 0x8b, 0xa9,
	0x79, 0xd6, 0x4a, 0xce, 0xa3, 0xc8, 0x27, 0xdc, 0xd5, 0x1d, 0x21, 0xed, 0x00, 0x00, 0x00, 0x1e,
	0x22, 0x16, 0x73, 0x68, 0x61, 0x6b, 0x61, 0x5f, 0x63, 0x65, 0x63, 0x32, 0x66, 0x36, 0x34, 0x61,
	0x61, 0x37, 0x38, 0x39, 0x30, 0x61, 0x31, 0x31, 0x48, 0xe3, 0xdc, 0x95, 0x9b, 0x06,
}

type DRM struct {
	boot atomic.Pointer[component.Progress]
}

var (
	once   sync.Once
	shared *DRM
)

func Get() *DRM { once.Do(func() { shared = &DRM{} }); return shared }

func (d *DRM) Name() string { return "drm" }

func (d *DRM) Startup() component.Progress {
	if p := d.boot.Load(); p != nil {
		return *p
	}
	return component.Progress{Doing: "checking"}
}

func (d *DRM) set(p component.Progress) { d.boot.Store(&p) }

func (d *DRM) Run(ctx context.Context) error {
	d.set(d.bring(ctx))
	<-ctx.Done()
	return nil
}

func (d *DRM) bring(ctx context.Context) component.Progress {
	c := display.Get().Helper()
	if c == nil {
		return component.Progress{Failed: true, Doing: "no display helper"}
	}
	began := time.Now()
	level, ready, err := provisioned(c)
	slog.Info(
		"widevine check",
		"level",
		level,
		"provisioned",
		ready,
		"took",
		time.Since(began),
		"err",
		err,
	)
	if err != nil {
		return component.Progress{Failed: true, Doing: err.Error()}
	}
	if level != "L1" {
		return component.Progress{
			Failed: true,
			Doing:  "Widevine " + level + ", no hardware decryption",
		}
	}
	if ready {
		return component.Progress{Done: true, Doing: "provisioned"}
	}

	d.set(component.Progress{Doing: "waiting for the network"})
	if !dhcp.Get().Wait(ctx) {
		return component.Progress{Failed: true, Doing: "no network"}
	}
	d.set(component.Progress{Doing: "provisioning"})
	if err := provision(ctx, c); err != nil {
		slog.Warn("widevine provisioning failed", "err", err)
		return component.Progress{Failed: true, Doing: err.Error()}
	}
	slog.Info("widevine provisioned")
	return component.Progress{Done: true, Doing: "provisioned"}
}

func provisioned(c *surface.Client) (string, bool, error) {
	if err := c.DRMOpen(checkSession, surface.Widevine, false, nil); err != nil {
		return "", false, fmt.Errorf("widevine: %w", err)
	}
	defer c.DRMClose(checkSession)
	level, err := c.DRMProperty(checkSession, "securityLevel")
	if err != nil {
		return "", false, fmt.Errorf("widevine security level: %w", err)
	}
	_, err = c.DRMRequest(checkSession, probe)
	return level, err == nil, nil
}

func provision(ctx context.Context, c *surface.Client) error {
	req, url, err := c.DRMProvision(surface.Widevine)
	if err != nil {
		return fmt.Errorf("provision request: %w", err)
	}
	if url == "" {
		return errors.New("provision request: no server")
	}
	ctx, cancel := context.WithTimeout(ctx, provisionFor)
	defer cancel()
	hr, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url+"&signedRequest="+string(req),
		http.NoBody,
	)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(hr)
	if err != nil {
		return fmt.Errorf("provisioning server: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("provisioning server: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provisioning server: %s", resp.Status)
	}
	return c.DRMProvisioned(body)
}
