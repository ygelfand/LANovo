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
	NativeAPI int

	Name  string
	Model string
	SoC   SoC

	PanelWidth, PanelHeight int

	Motion bool

	MicMutesCamera bool

	CameraMirror bool

	UISize string

	Mounted int

	Touch string

	Buttons []Button

	Amp, MicADC *Chip

	SecureDecoders map[string]string

	MaxFPS int

	CameraWidth, CameraHeight int

	SubWidth, SubHeight int
}

type Chip struct {
	Bus    int
	Addr   uint16
	Enable int
	Reset  int
}

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

func Current() Board {
	if b := current.Load(); b != nil {
		return *b
	}
	return Blueberry
}
