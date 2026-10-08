package parts

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/layout"
)

//go:embed lanovod.rc
var initRC []byte

type Part struct {
	Name     string
	Path     string
	Mode     os.FileMode
	Services []string
	SoC      board.SoC
	Boot     bool

	data []byte
}

func (p Part) Data() []byte { return p.data }

func (p Part) For(soc board.SoC) bool { return p.SoC == "" || p.SoC == soc }

var (
	Lanovod = Part{
		Name:     "lanovod",
		Path:     layout.Binary,
		Mode:     0o755,
		Services: []string{layout.Service},
	}
	InitRC  = Part{Name: "service", Path: layout.InitRC, Mode: 0o644, Boot: true, data: initRC}
	Surface = Part{Name: "helper", Path: layout.Surface, Mode: 0o755,
		Services: []string{layout.SurfaceService}, data: surface}
	CamShim = Part{
		Name: "camera preload",
		Path: layout.CamShim,
		Mode: 0o644,
		Services: []string{
			layout.CameraHALService,
			layout.CameraServerService,
			layout.CameraService,
		},
		data: camshim,
	}
	Camera = Part{Name: "camera helper", Path: layout.Camera, Mode: 0o755,
		Services: []string{layout.CameraService}, data: camera}
)

func Carried() []Part { return []Part{InitRC, Surface, CamShim, Camera} }

func Ensure() (rebootPending bool) {
	soc := board.Current().SoC

	var stale []Part
	for _, p := range Carried() {
		if len(p.data) == 0 || !p.For(soc) {
			continue
		}
		if have, err := os.ReadFile(p.Path); err == nil && bytes.Equal(have, p.data) {
			continue
		}
		stale = append(stale, p)
	}
	if len(stale) == 0 {
		return false
	}

	restore, err := Writable()
	if err != nil {
		slog.Error("remounting to update parts failed", "err", err)
		return false
	}
	defer func() {
		if err := restore(); err != nil {
			slog.Error("remounting read-only failed", "err", err)
		}
	}()

	for _, p := range stale {
		if err := Replace(p.Path, p.data, p.Mode); err != nil {
			slog.Error("updating a part failed", "part", p.Name, "path", p.Path, "err", err)
			continue
		}
		slog.Info("part updated", "part", p.Name, "path", p.Path)
		for _, svc := range p.Services {
			if state, _ := prop.Local.Getprop("init.svc." + svc); state != "running" {
				continue
			}
			if err := prop.Restart(prop.Local, svc); err != nil {
				slog.Error("restarting a service failed", "service", svc, "err", err)
			}
		}
		rebootPending = rebootPending || p.Boot
	}
	return rebootPending
}

func Replace(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := CopyLabel(layout.Binary, tmp); err != nil {
		slog.Warn("labelling failed", "path", tmp, "err", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("parts: replacing %s: %w", path, err)
	}
	return nil
}
