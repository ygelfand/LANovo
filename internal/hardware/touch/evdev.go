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

// The event types and axes this driver sends. Protocol B: a contact is a slot, and a slot holds a
// tracking id until it goes to -1.
const (
	evSyn = 0x00
	evAbs = 0x03

	synReport = 0x00

	absMTSlot       = 0x2f
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTTrackingID = 0x39
)

// released is the tracking id of a slot nobody is touching.
const released = -1

// rawEvent is the kernel's struct input_event. syscall.Timeval sizes itself per architecture,
// which is what makes this 16 bytes on the device and 24 on a host.
type rawEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// eventSize is what one event occupies on this architecture.
var eventSize = binary.Size(rawEvent{})

// read calls handle for every event until the file ends or fails.
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

// slot is what is known about one contact between reports.
type slot struct {
	id      int
	x, y    int
	touched bool
	changed bool

	// lifting holds the id until the Up has gone out, which is what matches it to its own Down.
	lifting bool
}

// decoder turns the event stream into contacts. The kernel sends only what changed, so the
// current position of a finger that is still moving is whatever it last said.
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

// event takes one event and returns the contacts to report, which is nothing until SYN_REPORT
// says the batch is complete.
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

// report drains what changed since the last one.
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
