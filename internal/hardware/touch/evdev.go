package touch

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
)

// Multi-touch protocol B: a slot holds a tracking id until it goes to -1.
const (
	evSyn = 0x00
	evAbs = 0x03

	synReport = 0x00

	absMTSlot       = 0x2f
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTTrackingID = 0x39
)

const released = -1

// struct input_event.
type rawEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

var eventSize = binary.Size(rawEvent{})

func read(f *os.File, handle func(rawEvent)) error {
	r := bufio.NewReaderSize(f, eventSize*64)

	for {
		var e rawEvent
		if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "file already closed") {
				return nil
			}
			return fmt.Errorf("touch: reading events: %w", err)
		}
		handle(e)
	}
}

type slot struct {
	id      int
	x, y    int
	touched bool
	changed bool

	lifting bool
}

type decoder struct {
	current int
	slots   map[int]*slot
}

func (d *decoder) at(n int) *slot {
	if d.slots == nil {
		d.slots = map[int]*slot{}
	}
	if d.slots[n] == nil {
		d.slots[n] = &slot{id: released}
	}
	return d.slots[n]
}

func (d *decoder) event(e rawEvent) []Contact {
	switch e.Type {
	case evAbs:
		d.abs(e.Code, int(e.Value))
	case evSyn:
		if e.Code == synReport {
			return d.report(time.Now())
		}
	}
	return nil
}

func (d *decoder) abs(code uint16, value int) {
	switch code {
	case absMTSlot:
		d.current = value
		return
	case absMTTrackingID:
		s := d.at(d.current)
		s.changed = true
		if value == released {
			s.lifting = true
		} else {
			s.id, s.lifting = value, false
			s.touched = false
		}
	case absMTPositionX:
		s := d.at(d.current)
		s.x, s.changed = value, true
	case absMTPositionY:
		s := d.at(d.current)
		s.y, s.changed = value, true
	}
}

func (d *decoder) report(now time.Time) []Contact {
	var out []Contact

	for n, s := range d.slots {
		if !s.changed {
			continue
		}
		s.changed = false

		x, y := rotate(s.x, s.y)
		switch {
		case s.lifting:
			if s.touched {
				out = append(out, Contact{Slot: n, ID: s.id, X: x, Y: y, Phase: Up, At: now})
			}
			s.id, s.touched, s.lifting = released, false, false
		case s.id == released:
		case !s.touched:
			s.touched = true
			out = append(out, Contact{Slot: n, ID: s.id, X: x, Y: y, Phase: Down, At: now})
		default:
			out = append(out, Contact{Slot: n, ID: s.id, X: x, Y: y, Phase: Move, At: now})
		}
	}
	return out
}
