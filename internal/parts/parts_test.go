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
			absent: []string{"ro.logd.auditd", "lanovo_widevine"},
		},
		{
			name:  "x501f",
			paths: board.X501F.Paths(),
			want: []string{
				"service lanovod /system/bin/sh /data/misc/lanovo/bin/launch-lanovod.sh",
				"on property:sys.boot_completed=1\n    start lanovo_wait",
				"on property:lanovo.takeover=1\n    mkdir /data/misc/lanovo",
				"exec u:r:su:s0 root root -- /system/bin/setenforce 0",
			},
			absent: []string{"on zygote-start", "stop audioserver", "stop audio-hal-2-0"},
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
				"service lanovo_widevine /data/misc/lanovo/bin/android.hardware.drm@1.0-service.widevine",
				"setenv LD_LIBRARY_PATH /data/misc/lanovo/lib:/vendor/lib:/system/lib",
				"on post-fs-data\n    start lanovo_widevine",
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
			got = strings.ReplaceAll(got, "\r\n", "\n")
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

func TestX501FTakeoverDoesNotLeakToOtherBoards(t *testing.T) {
	for _, b := range board.All() {
		t.Run(b.Name, func(t *testing.T) {
			rc, err := RC(b.Paths())
			if err != nil {
				t.Fatal(err)
			}
			x501f := b.Name == board.X501F.Name
			for _, directive := range []string{"setenv COUNTERTOP_X501F 1", "stop bootanim", "lanovo.takeover=1", "start audioserver", "start audio-hal-2-0"} {
				if strings.Contains(string(rc), directive) != x501f {
					t.Errorf("unexpected presence of %q for %s", directive, b.Name)
				}
			}
			for _, directive := range []string{"stop audioserver", "stop audio-hal-2-0", "on zygote-start"} {
				if strings.Contains(string(rc), directive) == x501f {
					t.Errorf("unexpected presence of %q for %s", directive, b.Name)
				}
			}
			wifiStops := 1 // The upstream msm8953-specific action.
			if x501f {
				wifiStops++
			}
			if got := strings.Count(string(rc), "stop wpa_supplicant"); got != wifiStops {
				t.Errorf("stop wpa_supplicant occurs %d times, want %d", got, wifiStops)
			}
		})
	}
}
