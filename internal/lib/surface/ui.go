package surface

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall"
)

const (
	opUIOpen    = 30
	opUIProgram = 31
	opUITexture = 32
	opUIFrame   = 33
	opUIRead    = 34

	uiQuadFloats = 36
	uiRunWords   = 40
)

func (c *Client) UIOpen(id uint32, w, h, z int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opUIOpen, id, uint32(w), uint32(h), uint32(z)); err != nil {
		return err
	}
	ans, _, err := c.recv(opUIOpen)
	if err != nil {
		return err
	}
	if len(ans) != 2 {
		return fmt.Errorf("surface: short ui reply")
	}
	if st := Status(ans[1]); st != 0 {
		return st
	}
	return nil
}

func (c *Client) UIProgram(id uint32, slot int, vs, fs string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.sendData(opUIProgram, []byte(vs+fs), id, uint32(slot), uint32(len(vs)), uint32(len(fs))); err != nil {
		return err
	}
	return c.status(opUIProgram)
}

func (c *Client) UITexture(id, tex uint32, w, h, x, y, rw, rh int, pix []byte) error {
	if len(pix) != rw*rh*4 {
		return Status(1)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendData(opUITexture, pix, id, tex, uint32(w), uint32(h), uint32(x), uint32(y), uint32(rw), uint32(rh))
}

func (c *Client) UIFrame(id uint32, rot int, clear [4]float32, quads []float32, runs []uint32) (submitUS, finishUS uint32, err error) {
	if len(quads)%uiQuadFloats != 0 || len(runs)%uiRunWords != 0 {
		return 0, 0, Status(1)
	}
	data := make([]byte, 4*(len(quads)+len(runs)))
	for i, f := range quads {
		binary.LittleEndian.PutUint32(data[4*i:], math.Float32bits(f))
	}
	for i, r := range runs {
		binary.LittleEndian.PutUint32(data[4*(len(quads)+i):], r)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	err = c.sendData(opUIFrame, data, id,
		math.Float32bits(clear[0]), math.Float32bits(clear[1]), math.Float32bits(clear[2]), math.Float32bits(clear[3]),
		uint32(rot), uint32(len(quads)/uiQuadFloats), uint32(len(runs)/uiRunWords))
	if err != nil {
		return 0, 0, err
	}
	ans, _, err := c.recv(opUIFrame)
	if err != nil {
		return 0, 0, err
	}
	if len(ans) != 4 {
		return 0, 0, fmt.Errorf("surface: short ui frame reply")
	}
	if st := Status(ans[0]); st != 0 {
		return 0, 0, st
	}
	return ans[2], ans[3], nil
}

func (c *Client) UIRead(id uint32) ([]byte, int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(opUIRead, id); err != nil {
		return nil, 0, 0, err
	}
	ans, fd, err := c.recv(opUIRead)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(ans) < 2 {
		closeFD(fd)
		return nil, 0, 0, fmt.Errorf("surface: short ui read reply")
	}
	if st := Status(ans[1]); st != 0 {
		closeFD(fd)
		return nil, 0, 0, st
	}
	if fd < 0 || len(ans) < 4 {
		closeFD(fd)
		return nil, 0, 0, fmt.Errorf("surface: ui read sent no pixels")
	}
	w, h := int(ans[2]), int(ans[3])
	m, err := syscall.Mmap(fd, 0, w*h*4, syscall.PROT_READ, syscall.MAP_SHARED)
	syscall.Close(fd)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("surface: mapping ui read of %d: %w", id, err)
	}
	pix := append([]byte(nil), m...)
	syscall.Munmap(m)
	return pix, w, h, nil
}

type UILayer struct {
	C  *Client
	ID uint32
}

func (l UILayer) Program(slot int, vs, fs string) error { return l.C.UIProgram(l.ID, slot, vs, fs) }

func (l UILayer) Texture(id uint32, w, h, x, y, rw, rh int, pix []byte) error {
	return l.C.UITexture(l.ID, id, w, h, x, y, rw, rh, pix)
}

func (l UILayer) Frame(rot int, clear [4]float32, quads []float32, runs []uint32) (uint32, uint32, error) {
	return l.C.UIFrame(l.ID, rot, clear, quads, runs)
}
