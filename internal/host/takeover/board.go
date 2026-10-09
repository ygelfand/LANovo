package takeover

import (
	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/board"
)

func DetectBoard(d *adb.Device) (board.Board, error) {
	f, err := board.Read(d)
	if err != nil {
		return board.Board{}, err
	}
	return board.Detect(f)
}
