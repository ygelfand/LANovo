package qcomaudio

import (
	"embed"
	"fmt"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

//go:embed tables/*.bin
var tables embed.FS

func Table(name string) ([][2]byte, error) {
	raw, err := tables.ReadFile("tables/" + name + ".bin")
	if err != nil {
		return nil, fmt.Errorf("qcomaudio: no amplifier table %q: %w", name, err)
	}
	return i2c.Pairs(raw)
}

// The I2S clocks have to be running while the amplifier loads.
func AmpOn(c board.Chip) error {
	rows, err := Table(c.Table)
	if err != nil {
		return err
	}
	return i2c.WriteRows(c.Bus, c.Addr, rows)
}
