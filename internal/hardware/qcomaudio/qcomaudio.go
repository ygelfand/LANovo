package qcomaudio

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

//go:embed amp.bin
var amp []byte

var Stock = sync.OnceValues(func() ([][2]byte, error) { return i2c.Pairs(amp) })

// The I2S clocks have to be running while the amplifier loads.
func AmpOn(c board.Chip) error {
	rows, err := Stock()
	if err != nil {
		return fmt.Errorf("qcomaudio: %w", err)
	}
	return i2c.WriteRows(c.Bus, c.Addr, rows)
}
