package parts

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/layout"
)

func TestRC(t *testing.T) {
	tests := []struct {
		name   string
		paths  layout.Paths
		camera bool
		want   []string
		absent []string
	}{
		{
			name:   "system",
			paths:  board.Blueberry.Paths(),
			camera: true,
			want: []string{
				"service lanovod /system/bin/lanovod run",
				"service lanovo_surface /system/bin/lanovo-surface",
				"setenv LD_PRELOAD /system/lib/liblanovo-camshim.so",
				"service lanovo_camera /system/bin/lanovo-camera",
				"group root system graphics input audio inet net_raw net_admin\n    seclabel u:r:su:s0\n    disabled",
			},
			absent: []string{"ro.logd.auditd", "lanovo_widevine"},
		},
		{
			name:   "data",
			paths:  board.Penghu.Paths(),
			camera: true,
			want: []string{
				"service lanovod /data/misc/lanovo/bin/lanovod run",
				"service lanovo_surface /data/misc/lanovo/bin/lanovo-surface",
				"setenv LD_PRELOAD /data/misc/lanovo/lib/liblanovo-camshim.so",
				"service lanovo_camera /data/misc/lanovo/bin/lanovo-camera",
				"group root system graphics input audio inet net_raw net_admin\n    seclabel u:r:init:s0\n    disabled",
				"on early-init\n    setprop ro.logd.auditd false",
				"service lanovo_widevine /data/misc/lanovo/bin/android.hardware.drm@1.0-service.widevine",
				"setenv LD_LIBRARY_PATH /data/misc/lanovo/lib:/vendor/lib:/system/lib",
				"on post-fs-data\n    start lanovo_widevine",
			},
			absent: []string{"/system/bin/lanovo", "/system/lib/liblanovo"},
		},
		{
			name:   "data without a camera",
			paths:  board.Cube.Paths(),
			camera: false,
			want: []string{
				"service lanovod /data/misc/lanovo/bin/lanovod run",
				"service lanovo_surface /data/misc/lanovo/bin/lanovo-surface",
				"service lanovo_mediacodec /vendor/bin/hw/android.hardware.media.omx@1.0-service",
				"setenv LD_LIBRARY_PATH /data/misc/lanovo_codec:/vendor/lib\n    disabled",
				"on post-fs-data && property:ro.product.device=mt8167s_som\n" +
					"    stop mediacodec\n    start lanovo_mediacodec",
			},
			absent: []string{
				"lanovo_camera", "lanovo_camserver", "lanovo_camhal",
				"property:ro.board.platform=mt8167",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := RC(tt.paths, tt.camera)
			if err != nil {
				t.Fatalf("RC(%+v) = %v", tt.paths, err)
			}
			got := string(rc)
			if strings.Contains(got, "{{") {
				t.Errorf("RC(%s) left a template action unrendered", tt.name)
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("RC(%s) lacks %q", tt.name, w)
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(got, a) {
					t.Errorf("RC(%s) still names %q", tt.name, a)
				}
			}
		})
	}
}

func TestCarriedLeavesTheBootImageRC(t *testing.T) {
	for _, tt := range []struct {
		name       string
		paths      layout.Paths
		camera     bool
		wantRC     bool
		wantCamera bool
	}{
		{name: "system", paths: board.Blueberry.Paths(), camera: true, wantRC: true, wantCamera: true},
		{name: "data", paths: board.Penghu.Paths(), camera: true, wantRC: false, wantCamera: true},
		{name: "data without a camera", paths: board.Cube.Paths(), camera: false, wantRC: false, wantCamera: false},
	} {
		carried, err := Carried(tt.paths, tt.camera)
		if err != nil {
			t.Fatalf("Carried(%s) = %v", tt.name, err)
		}
		hasRC := false
		hasCamera := false
		for _, p := range carried {
			if p.Name == InitRC.Name {
				hasRC = true
				if p.Path != tt.paths.InitRC {
					t.Errorf("Carried(%s) rc at %q, want %q", tt.name, p.Path, tt.paths.InitRC)
				}
			}
			if p.Name == Camera.Name || p.Name == CamShim.Name {
				hasCamera = true
			}
		}
		if hasRC != tt.wantRC {
			t.Errorf("Carried(%s) carries the rc = %v, want %v", tt.name, hasRC, tt.wantRC)
		}
		if hasCamera != tt.wantCamera {
			t.Errorf(
				"Carried(%s) carries camera parts = %v, want %v",
				tt.name,
				hasCamera,
				tt.wantCamera,
			)
		}
	}
}

func TestReplaceOpensADirectoryItCreatesWhateverTheUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	dir := filepath.Join(t.TempDir(), "codec")
	if err := Replace(filepath.Join(dir, "lib.so"), []byte("x"), 0o644, ""); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("created directory is %v, want 0755", got)
	}
}

func TestReplaceLeavesAnExistingDirectoryAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Replace(filepath.Join(dir, "file"), []byte("x"), 0o644, ""); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("existing directory became %v, want 0700", got)
	}
}

func TestTheCodecDriverIsCarriedToMediaTekBoardsAndStartsNothing(t *testing.T) {
	paths := board.Cube.Paths()
	carried, err := Carried(paths, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range carried {
		if p.Name != MTKDrvb.Name {
			continue
		}
		if p.Path != "/data/misc/lanovo_codec/libmtk_drvb.so" {
			t.Errorf("codec driver at %q", p.Path)
		}
		if len(p.Data()) == 0 {
			t.Error("codec driver is empty")
		}
		if len(p.Services) != 0 {
			t.Errorf("codec driver starts %v", p.Services)
		}
		if !p.For(board.MediaTek) || p.For(board.Qualcomm) {
			t.Errorf(
				"codec driver for MediaTek %v, Qualcomm %v",
				p.For(board.MediaTek),
				p.For(board.Qualcomm),
			)
		}
		return
	}
	t.Error("the codec driver is not carried")
}
