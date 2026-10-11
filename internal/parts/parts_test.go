package parts

import (
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/layout"
)

func TestRC(t *testing.T) {
	tests := []struct {
		name   string
		paths  layout.Paths
		want   []string
		absent []string
	}{
		{
			name:  "system",
			paths: board.Blueberry.Paths(),
			want: []string{
				"service lanovod /system/bin/lanovod run",
				"service lanovo_surface /system/bin/lanovo-surface",
				"setenv LD_PRELOAD /system/lib/liblanovo-camshim.so",
				"service lanovo_camera /system/bin/lanovo-camera",
				"group root system graphics input audio inet net_raw net_admin\n    seclabel u:r:su:s0\n    disabled",
			},
			absent: []string{"ro.logd.auditd"},
		},
		{
			name:  "data",
			paths: board.Penghu.Paths(),
			want: []string{
				"service lanovod /data/misc/lanovo/bin/lanovod run",
				"service lanovo_surface /data/misc/lanovo/bin/lanovo-surface",
				"setenv LD_PRELOAD /data/misc/lanovo/lib/liblanovo-camshim.so",
				"service lanovo_camera /data/misc/lanovo/bin/lanovo-camera",
				"group root system graphics input audio inet net_raw net_admin\n    seclabel u:r:init:s0\n    disabled",
				"on early-init\n    setprop ro.logd.auditd false",
			},
			absent: []string{"/system/bin/lanovo", "/system/lib/liblanovo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, err := RC(tt.paths)
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
		name   string
		paths  layout.Paths
		wantRC bool
	}{
		{name: "system", paths: board.Blueberry.Paths(), wantRC: true},
		{name: "data", paths: board.Penghu.Paths(), wantRC: false},
	} {
		carried, err := Carried(tt.paths)
		if err != nil {
			t.Fatalf("Carried(%s) = %v", tt.name, err)
		}
		hasRC := false
		for _, p := range carried {
			if p.Name == InitRC.Name {
				hasRC = true
				if p.Path != tt.paths.InitRC {
					t.Errorf("Carried(%s) rc at %q, want %q", tt.name, p.Path, tt.paths.InitRC)
				}
			}
		}
		if hasRC != tt.wantRC {
			t.Errorf("Carried(%s) carries the rc = %v, want %v", tt.name, hasRC, tt.wantRC)
		}
	}
}
