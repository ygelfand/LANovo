package board

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

type SoC string

const (
	Qualcomm SoC = "qualcomm"
	MediaTek SoC = "mediatek"
)

type Tessera struct {
	Board         string
	Columns, Rows int
}

type Board struct {
	NativeAPI int

	Name  string
	Model string
	SoC   SoC

	PanelWidth, PanelHeight int
	Diagonal                float64

	Motion bool

	MicMutesCamera bool

	CameraMirror bool

	CameraTurn int

	UISize string

	Tessera Tessera

	Mounted int

	Touch string

	Buttons []Button

	Amp, MicADC *Chip

	AmpPins []int

	Stereo bool

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

func ByName(name string) (Board, bool) {
	for _, b := range boards {
		if b.Name == name {
			return b, true
		}
	}
	return Board{}, false
}

func Resolve(s prop.Store, r Reader) (Board, error) {
	if name, err := s.Getprop(prop.Board); err == nil && name != "" {
		b, ok := ByName(name)
		if !ok {
			return Board{}, fmt.Errorf("board: %s=%q is not a board", prop.Board, name)
		}
		return b, nil
	}
	f, err := Read(r)
	if err != nil {
		return Board{}, err
	}
	return Detect(f)
}

type files struct{}

func (files) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

var (
	current   atomic.Pointer[Board]
	detection sync.Once
	detectErr error
)

func Set(b Board) { current.Store(&b) }

func Current() Board {
	detection.Do(detect)
	return *current.Load()
}

func Detected() error {
	detection.Do(detect)
	return detectErr
}

func detect() {
	b, err := Resolve(prop.Local, files{})
	if err != nil {
		b, detectErr = Blueberry, err
	}
	current.CompareAndSwap(nil, &b)
}
