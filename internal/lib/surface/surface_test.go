package surface

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type fake struct {
	t      *testing.T
	ln     *net.UnixListener
	frames chan []uint32
	scenes chan []uint32
	gl     chan []uint32
	drm    chan []uint32
	shared chan *os.File
	ver    uint32
}

func newFake(t *testing.T) (*fake, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "sf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	f := &fake{t: t, ln: ln, frames: make(chan []uint32, 8), scenes: make(chan []uint32, 8), gl: make(chan []uint32, 8), drm: make(chan []uint32, 8), shared: make(chan *os.File, 8), ver: version}
	t.Cleanup(func() { ln.Close() })
	go f.serve()
	return f, path
}

func (f *fake) serve() {
	c, err := f.ln.AcceptUnix()
	if err != nil {
		return
	}
	defer c.Close()
	for {
		op, words, ok := read(c)
		if !ok {
			return
		}
		switch op {
		case opHello:
			answer(c, opHello, nil, f.ver)
		case opCreate:
			id, w, h := words[0], words[3], words[4]
			file, _ := os.CreateTemp(f.t.TempDir(), "layer")
			file.Truncate(int64(w * h * 4))
			f.shared <- file
			answer(c, opCreate, syscall.UnixRights(int(file.Fd())), id, 0, w*4)
		case opFrame:
			f.frames <- words
			answer(c, opFrame, nil, words[0], words[1], 0)
		case opScene:
			f.scenes <- words
			answer(c, opScene, nil, 0)
		case opGLOpen:
			f.gl <- words
			answer(c, opGLOpen, nil, words[0], 0)
		case opGLTexture:
			f.gl <- words
			answer(c, opGLTexture, nil, 0)
		case opGLProgram:
			f.gl <- words
			answer(c, opGLProgram, nil, 7)
		case opGLValues:
			f.gl <- words
		case opDRMOpen, opDRMProvide, opAudioOpen:
			f.drm <- words
			answer(c, op, nil, words[0], 0)
		case opDRMRequest:
			f.drm <- words
			answerData(c, op, fakeChallenge, words[0], 0, uint32(len(fakeChallenge)))
		case opVideoCrypt:
			f.drm <- words
			answer(c, opVideoSample, nil, words[0], 0, 3, 1, 0)
		case opAudioSample:
			f.drm <- words
			answerData(c, op, fakePCM, words[0], 0, 48000, 2, uint32(len(fakePCM)))
		case opDRMClose, opAudioClose:
			f.drm <- words
		}
	}
}

func read(c *net.UnixConn) (uint32, []uint32, bool) {
	hdr := make([]byte, 8)
	if err := fill(c, hdr); err != nil {
		return 0, nil, false
	}
	body := make([]byte, binary.LittleEndian.Uint32(hdr[4:]))
	if err := fill(c, body); err != nil {
		return 0, nil, false
	}
	words := make([]uint32, len(body)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(body[4*i:])
	}
	return binary.LittleEndian.Uint32(hdr), words, true
}

var (
	fakeChallenge = []byte{9, 8, 7, 6, 5}
	fakePCM       = []byte{1, 0, 2, 0, 3, 0}
)

func answerData(c *net.UnixConn, op uint32, data []byte, words ...uint32) {
	pad := (len(data) + 3) &^ 3
	buf := make([]byte, 8+4*len(words)+pad)
	binary.LittleEndian.PutUint32(buf, op)
	binary.LittleEndian.PutUint32(buf[4:], uint32(4*len(words)+pad))
	for i, w := range words {
		binary.LittleEndian.PutUint32(buf[8+4*i:], w)
	}
	copy(buf[8+4*len(words):], data)
	c.Write(buf)
}

func wordBytes(words []uint32) []byte {
	b := make([]byte, 4*len(words))
	for i, w := range words {
		binary.LittleEndian.PutUint32(b[4*i:], w)
	}
	return b
}

func answer(c *net.UnixConn, op uint32, oob []byte, words ...uint32) {
	buf := make([]byte, 8+4*len(words))
	binary.LittleEndian.PutUint32(buf, op)
	binary.LittleEndian.PutUint32(buf[4:], uint32(4*len(words)))
	for i, w := range words {
		binary.LittleEndian.PutUint32(buf[8+4*i:], w)
	}
	c.WriteMsgUnix(buf, oob, nil)
}

func TestAnotherVersionIsRefused(t *testing.T) {
	f, path := newFake(t)
	f.ver = version + 1
	if _, err := Dial(path); !errors.Is(err, ErrVersion) {
		t.Errorf("err %v", err)
	}
}

func TestALayersPixelsAreSharedWithTheHelper(t *testing.T) {
	f, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	l, err := c.Create(7, 0, 0, 4, 2, 1, Opaque)
	if err != nil {
		t.Fatal(err)
	}
	if l.Stride != 16 || len(l.Pixels) != 32 {
		t.Fatalf("stride %d, %d bytes", l.Stride, len(l.Pixels))
	}
	l.Pixels[l.Stride+4] = 0xab
	got := make([]byte, 32)
	(<-f.shared).ReadAt(got, 0)
	if got[20] != 0xab {
		t.Errorf("the helper sees %x", got[20])
	}
}

func TestAFrameCarriesItsDamage(t *testing.T) {
	f, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Create(1, 0, 0, 10, 10, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.Frame(1, 5, []Rect{{1, 2, 3, 4}, {0, 0, 10, 1}}); err != nil {
		t.Fatal(err)
	}
	w := <-f.frames
	want := []uint32{1, 5, 2, 1, 2, 3, 4, 0, 0, 10, 1}
	if len(w) != len(want) {
		t.Fatalf("frame %v", w)
	}
	for i := range want {
		if w[i] != want[i] {
			t.Fatalf("frame %v, want %v", w, want)
		}
	}
}

func TestASceneCarriesPlacement(t *testing.T) {
	f, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Scene([]Placement{{ID: 3, X: -5, Y: 6, W: 7, H: 8, Z: 9, Visible: true, Alpha: 0.5}}); err != nil {
		t.Fatal(err)
	}
	w := <-f.scenes
	if len(w) != 9 || w[0] != 1 || w[1] != 3 || int32(w[2]) != -5 || w[7] != 1 || w[8] != 500 {
		t.Errorf("scene %v", w)
	}
}

func TestGLOpsCarryTheirPayloads(t *testing.T) {
	f, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.GLOpen(4, 300, 200, -3); err != nil {
		t.Fatal(err)
	}
	if w := <-f.gl; len(w) != 4 || w[0] != 4 || w[1] != 300 || int32(w[3]) != -3 {
		t.Errorf("open %v", w)
	}
	if err := c.GLTexture(4, 1, 2, 1, []byte{1, 2, 3, 4, 5, 6, 7, 8}); err != nil {
		t.Fatal(err)
	}
	if w := <-f.gl; len(w) != 6 || w[1] != 1 || w[2] != 2 || w[4] != 0x04030201 || w[5] != 0x08070605 {
		t.Errorf("texture %v", w)
	}
	if err := c.GLTexture(4, 0, 2, 2, make([]byte, 4)); err == nil {
		t.Error("a short texture was sent")
	}
	if err := c.GLProgram(4, WithLight, "void main(){}"); err != Status(7) {
		t.Errorf("program err %v", err)
	}
	if w := <-f.gl; len(w) != 7 || w[1] != WithLight || w[2] != 13 {
		t.Errorf("program %v", w)
	}
	if err := c.GLValues(4, []float32{0.5, -1}, Glow{Amount: 0.9, Radius: 0.035, Passes: 2}); err != nil {
		t.Fatal(err)
	}
	if w := <-f.gl; len(w) != 7 || w[1] != 2 || math.Float32frombits(w[2]) != 0.9 || w[4] != 2 ||
		math.Float32frombits(w[5]) != 0.5 || math.Float32frombits(w[6]) != -1 {
		t.Errorf("values %v", w)
	}
}

func TestADRMSessionCarriesItsSchemeCertificateAndChallenge(t *testing.T) {
	f, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cert := []byte{0xc0, 0xc1, 0xc2, 0xc3, 0xc4}
	if err := c.DRMOpen(7, Widevine, true, cert); err != nil {
		t.Fatal(err)
	}
	w := <-f.drm
	if w[0] != 7 || w[1] != 1 || w[2] != uint32(len(cert)) {
		t.Errorf("open %v", w[:3])
	}
	if b := wordBytes(w[3:]); !bytes.Equal(b[:16], Widevine[:]) || !bytes.Equal(b[16:16+len(cert)], cert) {
		t.Errorf("scheme and cert %x", b)
	}
	got, err := c.DRMRequest(7, []byte{1, 2, 3, 4, 5, 6})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fakeChallenge) {
		t.Errorf("challenge %x, want %x", got, fakeChallenge)
	}
	if w := <-f.drm; w[0] != 7 || w[1] != 6 || !bytes.Equal(wordBytes(w[2:])[:6], []byte{1, 2, 3, 4, 5, 6}) {
		t.Errorf("request %v", w)
	}
	if err := c.DRMProvide(7, []byte{0xaa}); err != nil {
		t.Fatal(err)
	}
	if w := <-f.drm; w[0] != 7 || w[1] != 1 || byte(w[2]) != 0xaa {
		t.Errorf("provide %v", w)
	}
}

func TestAnEncryptedSampleLaysOutItsCryptoBeforeItsData(t *testing.T) {
	f, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	k := &Crypt{Mode: CryptCENC, Subsamples: []Subsample{{Clear: 5, Encrypted: 11}, {Clear: 2, Encrypted: 0}}}
	for i := range 16 {
		k.Key[i], k.IV[i] = byte(i), byte(0x80+i)
	}
	data := []byte("0123456789abcdefgh")
	pl, err := c.VideoCrypt(3, 1500*time.Millisecond, SampleKey, k, data)
	if err != nil {
		t.Fatal(err)
	}
	if pl.Shown != 3 || pl.Dropped != 1 {
		t.Errorf("played %+v", pl)
	}
	w := <-f.drm
	us := uint64(1500 * time.Millisecond / time.Microsecond)
	if w[0] != 3 || w[1] != uint32(us) || w[2] != uint32(us>>32) || w[3] != SampleKey || w[4] != uint32(len(data)) {
		t.Errorf("head %v", w[:5])
	}
	if w[5] != CryptCENC || w[8] != 2 {
		t.Errorf("mode %d subsamples %d", w[5], w[8])
	}
	if b := wordBytes(w[9:17]); !bytes.Equal(b[:16], k.Key[:]) || !bytes.Equal(b[16:], k.IV[:]) {
		t.Errorf("key and iv %x", b)
	}
	if w[17] != 5 || w[18] != 11 || w[19] != 2 || w[20] != 0 {
		t.Errorf("subsamples %v", w[17:21])
	}
	if b := wordBytes(w[21:]); !bytes.Equal(b[:len(data)], data) {
		t.Errorf("data %q", b)
	}
}

func TestAnAudioSampleComesBackAsPCM(t *testing.T) {
	f, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.AudioOpen(5, 7, 44100, 2, []byte{0x12, 0x10}); err != nil {
		t.Fatal(err)
	}
	if w := <-f.drm; w[0] != 5 || w[1] != 7 || w[2] != 44100 || w[3] != 2 || w[4] != 2 {
		t.Errorf("open %v", w)
	}
	pcm, err := c.AudioSample(5, 0, 0, &Crypt{Mode: CryptCENC}, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pcm.Data, fakePCM) || pcm.Rate != 48000 || pcm.Channels != 2 {
		t.Errorf("pcm %+v", pcm)
	}
	<-f.drm
}

func TestAClearSampleSendsAClearCryptHeader(t *testing.T) {
	var k *Crypt
	w := k.words()
	if len(w) != 12 {
		t.Fatalf("%d words, want 12", len(w))
	}
	for i, v := range w {
		if v != 0 {
			t.Errorf("word %d is %d, want 0", i, v)
		}
	}
}

func TestAHelperHangingUpIsReportedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s")
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.AcceptUnix()
		if err != nil {
			return
		}
		read(c)
		answer(c, opHello, nil, version)
		c.Close()
	}()

	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	var drops []error
	c.Dropped.Listen(func(err error) { drops = append(drops, err) })

	for range 3 {
		c.Frame(1, 1, []Rect{{W: 1, H: 1}})
	}
	if len(drops) != 1 || c.Err() == nil || !errors.Is(c.Err(), drops[0]) {
		t.Fatalf("drops %v, err %v", drops, c.Err())
	}
	c.Close()
	if len(drops) != 1 {
		t.Errorf("closing reported %d drops", len(drops))
	}
}

func TestClosingIsNotADrop(t *testing.T) {
	_, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	dropped := false
	c.Dropped.Listen(func(error) { dropped = true })
	c.Close()
	if dropped || c.Err() == nil {
		t.Errorf("dropped %v, err %v", dropped, c.Err())
	}
}

func TestClosingTwiceIsQuiet(t *testing.T) {
	_, path := newFake(t)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}
