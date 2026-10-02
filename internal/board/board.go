// Package board is which Lenovo Smart Display this is, and what differs between them.
package board

import (
	"fmt"
	"sync/atomic"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

type SoC string

const (
	Qualcomm SoC = "qualcomm"
	MediaTek SoC = "mediatek"
)

type Board struct {
	Name  string
	Model string
	SoC   SoC

	// PanelWidth and PanelHeight are the native size, as the panel scans out.
	PanelWidth, PanelHeight int

	// Motion is whether there is an accelerometer.
	Motion bool

	CameraMirror bool

	UISize string

	// Mounted is the rotation from the panel's portrait, in degrees, when nothing says otherwise.
	Mounted int

	// Touch is the touchscreen's input device name; empty is not yet known.
	Touch string

	Buttons []Button

	// Amp and MicADC are codec parts driven over I2C from userspace; nil where the kernel owns them.
	Amp, MicADC *Chip

	// CameraWidth and CameraHeight are a preview size the vendor camera HAL lists.
	CameraWidth, CameraHeight int

	// SubWidth and SubHeight are a video size the vendor camera HAL lists; zero where it has no second stream.
	SubWidth, SubHeight int
}

// Chip is an I2C part with GPIOs for power and reset; a GPIO of 0 is none.
type Chip struct {
	Bus    int
	Addr   uint16
	Enable int
	Reset  int
}

// Button is a control on a sysfs GPIO. ActiveLow means it reads 0 when pressed or engaged.
type Button struct {
	Control   string
	GPIO      int
	ActiveLow bool
}

var boards []Board

func register(b Board) { boards = append(boards, b) }

func All() []Board { return append([]Board(nil), boards...) }

func ByModel(model string) (Board, bool) {
	for _, b := range boards {
		if b.Model == model {
			return b, true
		}
	}
	return Board{}, false
}

// Detect finds the board by its model property.
func Detect(s prop.Store) (Board, error) {
	model, err := s.Getprop(prop.Model)
	if err != nil {
		return Board{}, err
	}
	b, ok := ByModel(model)
	if !ok {
		return Board{}, fmt.Errorf("board: no board for model %q", model)
	}
	return b, nil
}

var current atomic.Pointer[Board]

func Set(b Board) { current.Store(&b) }

// Current is the board Set recorded, Blueberry until then.
func Current() Board {
	if b := current.Load(); b != nil {
		return *b
	}
	return Blueberry
}
