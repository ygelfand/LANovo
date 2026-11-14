package takeover

import (
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/board"
)

func DetectBoard(d *adb.Device, name string) (board.Board, error) {
	if name != "" {
		b, ok := board.ByName(name)
		if !ok {
			return board.Board{}, fmt.Errorf("no board %q", name)
		}
		if !b.Paths().X501F {
			return b, nil
		}
	}
	f, err := board.Read(d)
	if err != nil {
		return board.Board{}, err
	}
	b, err := board.Detect(f)
	if err == nil && name != "" && b.Name != name {
		return board.Board{}, fmt.Errorf("requested board %s, detected %s", name, b.Name)
	}
	return b, err
}
