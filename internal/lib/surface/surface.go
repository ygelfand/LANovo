package surface

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/hook"
)

const (
	Socket  = "/dev/socket/lanovo-surface"
	magic   = 0x53564e4c
	version = 4
)

const (
	opHello       = 1
	opCreate      = 2
	opFrame       = 3
	opScene       = 4
	opDestroy     = 5
	opVideoOpen   = 6
	opVideoSample = 7
	opVideoClock  = 8
	opVideoPlace  = 9
	opVideoClose  = 10
	opVideoFlush  = 11
	opGLOpen      = 12
	opGLTexture   = 13
	opGLProgram   = 14
	opGLValues    = 15
	opGLPoints    = 16
	opGLLines     = 17
	opGLQuads     = 18
	opGLOffscreen = 19
	opGLRead      = 20
	opDRMOpen     = 21
	opDRMRequest  = 22
	opDRMProvide  = 23
	opDRMClose    = 24
	opVideoCrypt  = 25
	opAudioOpen   = 26
	opAudioSample = 27
	opAudioClose  = 28
	opScreenRead  = 29

	opDRMProvision   = 35
	opDRMProvisioned = 36
)

const (
	CryptClear = 0
	CryptCENC  = 1
	CryptCBCS  = 2
)

var Widevine = [16]byte{0xed, 0xef, 0x8b, 0xa9, 0x79, 0xd6, 0x4a, 0xce, 0xa3, 0xc8, 0x27, 0xdc, 0xd5, 0x1d, 0x21, 0xed}

type Subsample struct{ Clear, Encrypted uint32 }

type Crypt struct {
	Mode       uint32
	Key, IV    [16]byte
	Subsamples []Subsample
	Encrypt    uint32
	Skip       uint32
}

func (k *Crypt) words() []uint32 {
	w := make([]uint32, 12)
	if k != nil {
		w[0], w[1], w[2], w[3] = k.Mode, k.Encrypt, k.Skip, uint32(len(k.Subsamples))
		for i := range 4 {
			w[4+i] = binary.LittleEndian.Uint32(k.Key[4*i:])
			w[8+i] = binary.LittleEndian.Uint32(k.IV[4*i:])
		}
		for _, s := range k.Subsamples {
			w = append(w, s.Clear, s.Encrypted)
		}
	}
	return w
}

type PCM struct {
	Data           []byte
	Rate, Channels int
}

const (
	VP9  = 1
	AVC  = 2
	HEVC = 3
	VP8  = 4
)

const (
	SampleKey = 1 << iota
	SampleEnd
)

type Played struct {
	Shown, Dropped uint32
	Ended          bool
}

type Matrix struct{ DsDx, DtDx, DtDy, DsDy float32 }

const (
	Opaque = 1 << iota
	Secure
)

var ErrVersion = errors.New("surface: the helper speaks another protocol version")

const Full Status = 6

type Status uint32

func (s Status) Error() string {
	switch s {
	case 1:
		return "surface: bad arguments"
	case 2:
		return "surface: no free layers"
	case 3:
		return "surface: SurfaceFlinger refused"
	case 4:
		return "surface: no shared memory"
	case 5:
		return "surface: the decoder refused"
	case 6:
		return "surface: the decoder is full"
	case 7:
		return "surface: the shader did not compile"
	case 8:
		return "surface: the DRM refused"
	}
	return fmt.Sprintf("surface: status %d", uint32(s))
}

type Rect struct{ X, Y, W, H int }

type Layer struct {
	ID     uint32
	Pixels []byte
	Stride int
	W, H   int
}

type Placement struct {
	ID         uint32
	X, Y, W, H int
	Z          int
	Visible    bool
	Alpha      float64
}

type Client struct {
	mu     sync.Mutex
	c      *net.UnixConn
	layers map[uint32]*Layer

	errMu sync.Mutex
	err   error

	Dropped hook.Hook[error]
}

func Dial(path string) (*Client, error) {
	c, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	cl := &Client{c: c, layers: map[uint32]*Layer{}}
	if err := cl.send(opHello, magic, version); err != nil {
		c.Close()
		return nil, err
	}
	words, _, err := cl.recv(opHello)
	if err != nil {
		c.Close()
		return nil, err
	}
	if len(words) != 1 || words[0] != version {
		c.Close()
		return nil, ErrVersion
	}
	return cl, nil
}

func DialWait(path string, wait time.Duration) (*Client, error) {
	deadline := time.Now().Add(wait)
	for {
		c, err := Dial(path)
		if err == nil || errors.Is(err, ErrVersion) || time.Now().After(deadline) {
			return c, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (c *Client) Err() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	return c.err
}

func (c *Client) drop(err error) error {
	if err == nil {
		return nil
	}
	c.errMu.Lock()
	first := c.err == nil
	if first {
		c.err = err
	}
	c.errMu.Unlock()
	if first {
		c.Dropped.Emit(err)
	}
	return err
}

func (c *Client) Close() error {
	c.errMu.Lock()
	if c.err == nil {
		c.err = net.ErrClosed
	}
	c.errMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.layers {
		syscall.Munmap(l.Pixels)
	}
	c.layers = nil
	if err := c.c.Close(); !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func (c *Client) Create(id uint32, x, y, w, h, z int, flags uint32) (*Layer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opCreate, id, uint32(x), uint32(y), uint32(w), uint32(h), uint32(z), flags); err != nil {
		return nil, err
	}
	words, fd, err := c.recv(opCreate)
	if err != nil {
		return nil, err
	}
	if len(words) < 2 {
		closeFD(fd)
		return nil, fmt.Errorf("surface: short create reply")
	}
	if st := Status(words[1]); st != 0 {
		closeFD(fd)
		return nil, st
	}
	if fd < 0 || len(words) < 3 {
		return nil, fmt.Errorf("surface: create sent no buffer")
	}
	stride := int(words[2])
	pix, err := syscall.Mmap(fd, 0, stride*h, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	syscall.Close(fd)
	if err != nil {
		return nil, fmt.Errorf("surface: mapping layer %d: %w", id, err)
	}
	l := &Layer{ID: id, Pixels: pix, Stride: stride, W: w, H: h}
	c.layers[id] = l
	return l, nil
}

func (c *Client) Frame(id, seq uint32, rects []Rect) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	words := make([]uint32, 0, 3+4*len(rects))
	words = append(words, id, seq, uint32(len(rects)))
	for _, r := range rects {
		words = append(words, uint32(r.X), uint32(r.Y), uint32(r.W), uint32(r.H))
	}
	if err := c.send(opFrame, words...); err != nil {
		return err
	}
	ans, _, err := c.recv(opFrame)
	if err != nil {
		return err
	}
	if len(ans) != 3 || ans[1] != seq {
		return fmt.Errorf("surface: frame reply out of step")
	}
	if st := Status(ans[2]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) Scene(ps []Placement) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	words := make([]uint32, 0, 1+8*len(ps))
	words = append(words, uint32(len(ps)))
	for _, p := range ps {
		vis := uint32(0)
		if p.Visible {
			vis = 1
		}
		alpha := uint32(max(0, min(1, p.Alpha)) * 1000)
		words = append(words, p.ID, uint32(p.X), uint32(p.Y), uint32(p.W), uint32(p.H), uint32(p.Z), vis, alpha)
	}
	if err := c.send(opScene, words...); err != nil {
		return err
	}
	ans, _, err := c.recv(opScene)
	if err != nil {
		return err
	}
	if len(ans) != 1 {
		return fmt.Errorf("surface: short scene reply")
	}
	if st := Status(ans[0]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) Destroy(id uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l := c.layers[id]; l != nil {
		syscall.Munmap(l.Pixels)
		delete(c.layers, id)
	}
	return c.send(opDestroy, id)
}

func MIME(codec uint32) string {
	switch codec {
	case VP9:
		return "video/x-vnd.on2.vp9"
	case AVC:
		return "video/avc"
	case HEVC:
		return "video/hevc"
	case VP8:
		return "video/x-vnd.on2.vp8"
	}
	return ""
}

func (c *Client) VideoOpen(id, codec uint32, w, h, z int, session uint32, decoder string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opVideoOpen, []byte(decoder), id, codec, uint32(w), uint32(h), uint32(z), session, uint32(len(decoder))); err != nil {
		return err
	}
	ans, _, err := c.recv(opVideoOpen)
	if err != nil {
		return err
	}
	if len(ans) != 2 {
		return fmt.Errorf("surface: short video reply")
	}
	if st := Status(ans[1]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) VideoSample(id uint32, pts time.Duration, flags uint32, data []byte) (Played, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	us := uint64(pts / time.Microsecond)
	if err := c.sendData(opVideoSample, data, id, uint32(us), uint32(us>>32), flags, uint32(len(data))); err != nil {
		return Played{}, err
	}
	return c.played()
}

func (c *Client) VideoCrypt(id uint32, pts time.Duration, flags uint32, k *Crypt, data []byte) (Played, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	us := uint64(pts / time.Microsecond)
	words := append([]uint32{id, uint32(us), uint32(us >> 32), flags, uint32(len(data))}, k.words()...)
	if err := c.sendData(opVideoCrypt, data, words...); err != nil {
		return Played{}, err
	}
	return c.played()
}

func (c *Client) played() (Played, error) {
	ans, _, err := c.recv(opVideoSample)
	if err != nil {
		return Played{}, err
	}
	if len(ans) != 5 {
		return Played{}, fmt.Errorf("surface: short sample reply")
	}
	p := Played{Shown: ans[2], Dropped: ans[3], Ended: ans[4] != 0}
	if st := Status(ans[1]); st != 0 {
		return p, st
	}
	return p, nil
}

func (c *Client) VideoClock(id uint32, at time.Duration, running bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	us := uint64(at / time.Microsecond)
	run := uint32(0)
	if running {
		run = 1
	}
	return c.send(opVideoClock, id, uint32(us), uint32(us>>32), run)
}

func (c *Client) VideoPlace(id uint32, x, y float32, m Matrix, z int, visible bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	vis := uint32(0)
	if visible {
		vis = 1
	}
	if err := c.send(opVideoPlace, id, math.Float32bits(x), math.Float32bits(y),
		math.Float32bits(m.DsDx), math.Float32bits(m.DtDx), math.Float32bits(m.DtDy), math.Float32bits(m.DsDy),
		uint32(z), vis); err != nil {
		return err
	}
	ans, _, err := c.recv(opVideoPlace)
	if err != nil {
		return err
	}
	if len(ans) != 1 {
		return fmt.Errorf("surface: short place reply")
	}
	if st := Status(ans[0]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) VideoClose(id uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send(opVideoClose, id, 0)
}

func (c *Client) VideoFlush(id uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opVideoFlush, id); err != nil {
		return err
	}
	ans, _, err := c.recv(opVideoFlush)
	if err != nil {
		return err
	}
	if len(ans) != 1 {
		return fmt.Errorf("surface: short flush reply")
	}
	if st := Status(ans[0]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) DRMOpen(id uint32, scheme [16]byte, privacy bool, cert []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	flags := uint32(0)
	if privacy {
		flags = 1
	}
	words := []uint32{id, flags, uint32(len(cert))}
	for i := range 4 {
		words = append(words, binary.LittleEndian.Uint32(scheme[4*i:]))
	}
	if err := c.sendData(opDRMOpen, cert, words...); err != nil {
		return err
	}
	return c.pairStatus(opDRMOpen)
}

func (c *Client) DRMRequest(id uint32, init []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opDRMRequest, init, id, uint32(len(init))); err != nil {
		return nil, err
	}
	ans, data, err := c.recvData(opDRMRequest, 3)
	if err != nil {
		return nil, err
	}
	if st := Status(ans[1]); st != 0 {
		return nil, st
	}
	return data, nil
}

func (c *Client) DRMProvide(id uint32, license []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opDRMProvide, license, id, uint32(len(license))); err != nil {
		return err
	}
	return c.pairStatus(opDRMProvide)
}

func (c *Client) DRMProvision(scheme [16]byte) ([]byte, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var words []uint32
	for i := range 4 {
		words = append(words, binary.LittleEndian.Uint32(scheme[4*i:]))
	}
	if err := c.send(opDRMProvision, words...); err != nil {
		return nil, "", err
	}
	ans, data, err := c.recvData(opDRMProvision, 3)
	if err != nil {
		return nil, "", err
	}
	if st := Status(ans[0]); st != 0 {
		return nil, "", st
	}
	n := int(ans[1])
	if n > len(data) {
		return nil, "", fmt.Errorf("surface: provision request of %d bytes in a %d byte reply", n, len(data))
	}
	return data[:n], string(data[n:]), nil
}

func (c *Client) DRMProvisioned(response []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opDRMProvisioned, response, uint32(len(response))); err != nil {
		return err
	}
	ans, _, err := c.recv(opDRMProvisioned)
	if err != nil {
		return err
	}
	if len(ans) != 1 {
		return fmt.Errorf("surface: short provision reply")
	}
	if st := Status(ans[0]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) DRMClose(id uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send(opDRMClose, id)
}

func (c *Client) AudioOpen(id, session uint32, rate, channels int, config []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opAudioOpen, config, id, session, uint32(rate), uint32(channels), uint32(len(config))); err != nil {
		return err
	}
	return c.pairStatus(opAudioOpen)
}

func (c *Client) AudioSample(id uint32, pts time.Duration, flags uint32, k *Crypt, data []byte) (PCM, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	us := uint64(pts / time.Microsecond)
	words := append([]uint32{id, uint32(us), uint32(us >> 32), flags, uint32(len(data))}, k.words()...)
	if err := c.sendData(opAudioSample, data, words...); err != nil {
		return PCM{}, err
	}
	ans, pcm, err := c.recvData(opAudioSample, 5)
	if err != nil {
		return PCM{}, err
	}
	out := PCM{Data: pcm, Rate: int(ans[2]), Channels: int(ans[3])}
	if st := Status(ans[1]); st != 0 {
		return out, st
	}
	return out, nil
}

func (c *Client) AudioClose(id uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send(opAudioClose, id)
}

func (c *Client) pairStatus(op uint32) error {
	ans, _, err := c.recv(op)
	if err != nil {
		return err
	}
	if len(ans) != 2 {
		return fmt.Errorf("surface: short reply to op %d", op)
	}
	if st := Status(ans[1]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) GLOpen(id uint32, w, h, z int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opGLOpen, id, uint32(w), uint32(h), uint32(z)); err != nil {
		return err
	}
	ans, _, err := c.recv(opGLOpen)
	if err != nil {
		return err
	}
	if len(ans) != 2 {
		return fmt.Errorf("surface: short gl reply")
	}
	if st := Status(ans[1]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) GLOffscreen(id uint32, w, h int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opGLOffscreen, id, uint32(w), uint32(h)); err != nil {
		return err
	}
	ans, _, err := c.recv(opGLOffscreen)
	if err != nil {
		return err
	}
	if len(ans) != 2 {
		return fmt.Errorf("surface: short gl reply")
	}
	if st := Status(ans[1]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) GLRead(id uint32) ([]byte, int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opGLRead, id); err != nil {
		return nil, 0, 0, err
	}
	ans, fd, err := c.recv(opGLRead)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(ans) < 2 {
		closeFD(fd)
		return nil, 0, 0, fmt.Errorf("surface: short read reply")
	}
	if st := Status(ans[1]); st != 0 {
		closeFD(fd)
		return nil, 0, 0, st
	}
	if fd < 0 || len(ans) < 4 {
		closeFD(fd)
		return nil, 0, 0, fmt.Errorf("surface: read sent no pixels")
	}
	w, h := int(ans[2]), int(ans[3])
	m, err := syscall.Mmap(fd, 0, w*h*4, syscall.PROT_READ, syscall.MAP_SHARED)
	syscall.Close(fd)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("surface: mapping read of %d: %w", id, err)
	}
	pix := append([]byte(nil), m...)
	syscall.Munmap(m)
	return pix, w, h, nil
}

func (c *Client) GLTexture(id uint32, unit, w, h int, rgba []byte) error {
	if len(rgba) != w*h*4 {
		return Status(1)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opGLTexture, rgba, id, uint32(unit), uint32(w), uint32(h)); err != nil {
		return err
	}
	return c.status(opGLTexture)
}

const (
	WithLight = 1 << iota
	WithPre
	WithFeed
	FeedHalf
	WithSplat
	SplatHalf
	WithLines
	FeedFloat
)

type Glow struct {
	Amount, Radius float32
	Passes         int
}

func (c *Client) GLProgram(id uint32, flags uint32, src string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opGLProgram, []byte(src), id, flags, uint32(len(src))); err != nil {
		return err
	}
	return c.status(opGLProgram)
}

func (c *Client) GLValues(id uint32, u []float32, g Glow) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	words := make([]uint32, 5+len(u))
	words[0] = id
	words[1] = uint32(len(u))
	words[2] = math.Float32bits(g.Amount)
	words[3] = math.Float32bits(g.Radius)
	words[4] = uint32(g.Passes)
	for i, v := range u {
		words[5+i] = math.Float32bits(v)
	}
	return c.send(opGLValues, words...)
}

const PointFloats = 8

const LineFloats = 9

func (c *Client) GLLines(id uint32, segs []float32) error {
	if len(segs)%LineFloats != 0 {
		return Status(1)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	words := make([]uint32, 2+len(segs))
	words[0] = id
	words[1] = uint32(len(segs) / LineFloats)
	for i, v := range segs {
		words[2+i] = math.Float32bits(v)
	}
	return c.send(opGLLines, words...)
}

const QuadFloats = 12

func (c *Client) GLQuads(id uint32, quads []float32) error {
	if len(quads)%QuadFloats != 0 {
		return Status(1)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	words := make([]uint32, 2+len(quads))
	words[0] = id
	words[1] = uint32(len(quads) / QuadFloats)
	for i, v := range quads {
		words[2+i] = math.Float32bits(v)
	}
	return c.send(opGLQuads, words...)
}

func (c *Client) GLPoints(id uint32, pts []float32) error {
	if len(pts)%PointFloats != 0 {
		return Status(1)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	words := make([]uint32, 2+len(pts))
	words[0] = id
	words[1] = uint32(len(pts) / PointFloats)
	for i, v := range pts {
		words[2+i] = math.Float32bits(v)
	}
	return c.send(opGLPoints, words...)
}

func (c *Client) status(op uint32) error {
	ans, _, err := c.recv(op)
	if err != nil {
		return err
	}
	if len(ans) != 1 {
		return fmt.Errorf("surface: short reply to op %d", op)
	}
	if st := Status(ans[0]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) send(op uint32, words ...uint32) error {
	return c.sendData(op, nil, words...)
}

func (c *Client) sendData(op uint32, data []byte, words ...uint32) error {
	return c.drop(c.write(op, data, words...))
}

func (c *Client) write(op uint32, data []byte, words ...uint32) error {
	pad := (len(data) + 3) &^ 3
	head := make([]byte, 8+4*len(words), 8+4*len(words)+3)
	binary.LittleEndian.PutUint32(head[0:], op)
	binary.LittleEndian.PutUint32(head[4:], uint32(4*len(words)+pad))
	for i, w := range words {
		binary.LittleEndian.PutUint32(head[8+4*i:], w)
	}
	if len(data) == 0 {
		_, err := c.c.Write(head)
		return err
	}
	bufs := net.Buffers{head, data}
	if pad > len(data) {
		bufs = append(bufs, make([]byte, pad-len(data)))
	}
	_, err := bufs.WriteTo(c.c)
	return err
}

func (c *Client) recv(want uint32) ([]uint32, int, error) {
	words, fd, err := c.read(want)
	return words, fd, c.drop(err)
}

func (c *Client) read(want uint32) ([]uint32, int, error) {
	hdr := make([]byte, 8)
	oob := make([]byte, syscall.CmsgSpace(4))
	n, oobn, _, _, err := c.c.ReadMsgUnix(hdr, oob)
	if err != nil {
		return nil, -1, err
	}
	fd := -1
	if oobn > 0 {
		if msgs, err := syscall.ParseSocketControlMessage(oob[:oobn]); err == nil && len(msgs) > 0 {
			if fds, err := syscall.ParseUnixRights(&msgs[0]); err == nil && len(fds) > 0 {
				fd = fds[0]
			}
		}
	}
	if err := fill(c.c, hdr[n:]); err != nil {
		closeFD(fd)
		return nil, -1, err
	}
	op, size := binary.LittleEndian.Uint32(hdr[0:]), binary.LittleEndian.Uint32(hdr[4:])
	if op != want || size%4 != 0 || size > 64 {
		closeFD(fd)
		return nil, -1, fmt.Errorf("surface: unexpected reply %d (%d bytes) to %d", op, size, want)
	}
	body := make([]byte, size)
	if err := fill(c.c, body); err != nil {
		closeFD(fd)
		return nil, -1, err
	}
	words := make([]uint32, size/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(body[4*i:])
	}
	return words, fd, nil
}

func (c *Client) recvData(want uint32, n int) ([]uint32, []byte, error) {
	words, data, err := c.readData(want, n)
	return words, data, c.drop(err)
}

func (c *Client) readData(want uint32, n int) ([]uint32, []byte, error) {
	hdr := make([]byte, 8)
	if err := fill(c.c, hdr); err != nil {
		return nil, nil, err
	}
	op, size := binary.LittleEndian.Uint32(hdr[0:]), binary.LittleEndian.Uint32(hdr[4:])
	if op != want || size%4 != 0 || size < uint32(4*n) || size > 64<<20 {
		return nil, nil, fmt.Errorf("surface: unexpected reply %d (%d bytes) to %d", op, size, want)
	}
	body := make([]byte, size)
	if err := fill(c.c, body); err != nil {
		return nil, nil, err
	}
	words := make([]uint32, n)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(body[4*i:])
	}
	data := body[4*n:]
	if length := int(words[n-1]); length <= len(data) {
		data = data[:length]
	}
	return words, data, nil
}

func fill(c *net.UnixConn, b []byte) error {
	for len(b) > 0 {
		n, err := c.Read(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func closeFD(fd int) {
	if fd >= 0 {
		syscall.Close(fd)
	}
}

// ScreenRead is the screen as SurfaceFlinger composed it, every layer included, as tight RGBA.
func (c *Client) ScreenRead() ([]byte, int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opScreenRead); err != nil {
		return nil, 0, 0, err
	}
	ans, fd, err := c.recv(opScreenRead)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(ans) < 1 {
		closeFD(fd)
		return nil, 0, 0, fmt.Errorf("surface: short screen reply")
	}
	if st := Status(ans[0]); st != 0 {
		closeFD(fd)
		return nil, 0, 0, st
	}
	if fd < 0 || len(ans) < 3 {
		closeFD(fd)
		return nil, 0, 0, fmt.Errorf("surface: screen read sent no pixels")
	}
	w, h := int(ans[1]), int(ans[2])
	m, err := syscall.Mmap(fd, 0, w*h*4, syscall.PROT_READ, syscall.MAP_SHARED)
	syscall.Close(fd)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("surface: mapping the screen: %w", err)
	}
	pix := append([]byte(nil), m...)
	syscall.Munmap(m)
	return pix, w, h, nil
}
