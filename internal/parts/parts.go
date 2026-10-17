package parts

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/layout"
)

//go:embed lanovod.rc
var initRC string

var rcTemplate = template.Must(template.New("lanovod.rc").Option("missingkey=error").Parse(initRC))

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

func Voice() []byte { return voice }

func AirPlay() []byte { return airplay }

func (p Part) For(soc board.SoC) bool { return p.SoC == "" || p.SoC == soc }

func (p Part) at(path string) Part {
	p.Path = path
	return p
}

var (
	Lanovod = Part{
		Name:     "lanovod",
		Mode:     0o755,
		Services: []string{layout.Service},
	}
	InitRC  = Part{Name: "service", Mode: 0o644, Boot: true}
	Surface = Part{Name: "helper", Mode: 0o755,
		Services: []string{layout.SurfaceService}, data: surface}
	CamShim = Part{
		Name: "camera preload",
		Mode: 0o644,
		Services: []string{
			layout.CameraHALService,
			layout.CameraServerService,
			layout.CameraService,
		},
		data: camshim,
	}
	Camera = Part{Name: "camera helper", Mode: 0o755,
		Services: []string{layout.CameraService}, data: camera}
)

func RC(p layout.Paths) ([]byte, error) {
	var b bytes.Buffer
	if err := rcTemplate.Execute(&b, p); err != nil {
		return nil, fmt.Errorf("parts: rendering %s: %w", rcTemplate.Name(), err)
	}
	return b.Bytes(), nil
}

// A board with its root off /system carries the service definition in its boot image.
func Carried(p layout.Paths) ([]Part, error) {
	carried := []Part{Surface.at(p.Surface), CamShim.at(p.CamShim), Camera.at(p.Camera)}
	if !p.OnSystem() {
		return carried, nil
	}
	rc, err := RC(p)
	if err != nil {
		return nil, err
	}
	rcPart := InitRC.at(p.InitRC)
	rcPart.data = rc
	return append([]Part{rcPart}, carried...), nil
}

func Ensure() (rebootPending bool) {
	b := board.Current()
	paths := b.Paths()
	carried, err := Carried(paths)
	if err != nil {
		slog.Error("listing parts failed", "err", err)
		return false
	}

	var stale []Part
	for _, p := range carried {
		if len(p.data) == 0 || !p.For(b.SoC) {
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

	restore, err := Writable(paths)
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
		if err := Replace(p.Path, p.data, p.Mode, paths.Binary); err != nil {
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

func Replace(path string, data []byte, mode os.FileMode, labelFrom string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := CopyLabel(labelFrom, tmp); err != nil {
		slog.Warn("labelling failed", "path", tmp, "err", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("parts: replacing %s: %w", path, err)
	}
	return nil
}
