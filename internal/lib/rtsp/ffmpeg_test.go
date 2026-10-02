package rtsp

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func accessUnits(b []byte) [][]byte {
	var aus [][]byte
	var cur []byte
	for _, n := range NALUs(b) {
		if nalType(n) == nalAUD && len(cur) > 0 {
			aus = append(aus, cur)
			cur = nil
		}
		cur = append(cur, 0, 0, 0, 1)
		cur = append(cur, n...)
	}
	if len(cur) > 0 {
		aus = append(aus, cur)
	}
	return aus
}

func TestFFmpegPlaysIt(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no ffmpeg")
	}
	file := filepath.Join(t.TempDir(), "src.h264")
	gen := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=15", "-frames:v", "45",
		"-c:v", "libx264", "-g", "15", "-bf", "0", "-x264-params", "aud=1", "-f", "h264", file)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot make H.264 here: %v %s", err, out)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	aus := accessUnits(raw)
	if len(aus) < 30 {
		t.Fatalf("%d access units", len(aus))
	}

	srv := NewServer()
	st := NewStream()
	srv.Handle("main", st)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	defer srv.Close()

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		tick := time.NewTicker(time.Second / 15)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			st.Write(aus[i%len(aus)], time.Duration(i)*time.Second/15)
		}
	}()

	for _, transport := range []string{"tcp", "udp"} {
		t.Run(transport, func(t *testing.T) {
			run, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(run, "ffmpeg", "-v", "error", "-rtsp_transport", transport,
				"-i", "rtsp://"+ln.Addr().String()+"/main", "-frames:v", "20", "-f", "null", "-")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("ffmpeg: %v\n%s", err, out)
			}
			if s := strings.TrimSpace(string(out)); s != "" {
				t.Errorf("ffmpeg complained:\n%s", s)
			}
		})
	}
}
