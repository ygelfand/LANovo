package mtkcamera

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fake(t *testing.T, status uint32, frames [][]byte) chan []uint32 {
	t.Helper()
	dir, err := os.MkdirTemp("", "mc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	old := socket
	socket = path
	t.Cleanup(func() { socket = old })

	got := make(chan []uint32, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		req := make([]uint32, 12)
		binary.Read(c, binary.LittleEndian, req)
		extra := make([]byte, req[11])
		io.ReadFull(c, extra)
		got <- append(req, uint32(len(string(extra))))
		binary.Write(c, binary.LittleEndian, []uint32{status, req[2], req[3], req[4]})
		for i, f := range frames {
			flags := uint32(0)
			if i == 0 {
				flags = frameConfig
			}
			if i == 2 {
				flags = frameSub
			}
			pts := uint64(i) * 33333
			binary.Write(c, binary.LittleEndian, []uint32{uint32(len(f)), flags, uint32(pts), uint32(pts >> 32)})
			c.Write(f)
		}
		time.Sleep(200 * time.Millisecond)
	}()
	return got
}

func TestOpenAsksForWhatWasConfiguredAndFramesComeBackWhole(t *testing.T) {
	got := fake(t, 0, [][]byte{{0, 0, 0, 1, 0x67}, {0, 0, 0, 1, 0x65, 1, 2, 3}, {0, 0, 0, 1, 0x65}})

	s, err := Open(Config{Width: 1280, Height: 720, FPS: 30, Bitrate: 4000000, Keyframe: 2,
		SubWidth: 640, SubHeight: 480, SubBitrate: 1000000, Params: "whitebalance=daylight", Turn: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	req := <-got
	if req[0] != magic || req[1] != version || req[2] != 1280 || req[3] != 720 || req[4] != 30 || req[5] != 4000000 || req[6] != 2 ||
		req[7] != 640 || req[8] != 480 || req[9] != 1000000 || req[10] != 3 || req[11] != 21 {
		t.Errorf("asked for %v", req)
	}

	first, err := s.Next(time.Second)
	if err != nil || !first.Config || len(first.Data) != 5 {
		t.Fatalf("first %+v %v", first, err)
	}
	second, err := s.Next(time.Second)
	if err != nil || second.Config || second.Sub || len(second.Data) != 8 || second.PTS != 33333*time.Microsecond {
		t.Fatalf("second %+v %v", second, err)
	}
	third, err := s.Next(time.Second)
	if err != nil || !third.Sub {
		t.Fatalf("third %+v %v", third, err)
	}
	if _, err := s.Next(50 * time.Millisecond); !Timeout(err) {
		t.Errorf("waiting on a quiet stream gave %v, want a timeout", err)
	}
}

func TestARefusalSaysWhy(t *testing.T) {
	fake(t, 2, nil)
	_, err := Open(Config{Width: 1280, Height: 720, FPS: 30, Bitrate: 1, Keyframe: 2})
	if err == nil || !strings.Contains(err.Error(), "camera refused") {
		t.Errorf("err %v", err)
	}
}

func TestAnIDRSliceIsAKeyFrameWhateverTheFlagSays(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"idr after sps and pps", []byte{0, 0, 0, 1, 0x67, 1, 2, 0, 0, 0, 1, 0x68, 3, 0, 0, 1, 0x65, 9}, true},
		{"idr alone", []byte{0, 0, 1, 0x65, 1}, true},
		{"p slice", []byte{0, 0, 0, 1, 0x41, 1, 2, 3}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := idr(c.data); got != c.want {
			t.Errorf("%s: idr = %v, want %v", c.name, got, c.want)
		}
	}
}
