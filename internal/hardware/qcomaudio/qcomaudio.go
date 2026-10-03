// Package qcomaudio brings up the amplifier the stock firmware drives from its OEM app on the
// Qualcomm board, programmed over I2C with the register table that app carries.
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

// Stock is the embedded amplifier table, parsed once.
var Stock = sync.OnceValues(func() ([][2]byte, error) { return i2c.Pairs(amp) })

// AmpOn loads the amplifier. The I2S clocks have to be running while it loads.
func AmpOn(c board.Chip) error {
	rows, err := Stock()
	if err != nil {
		return fmt.Errorf("qcomaudio: %w", err)
	}
	return i2c.WriteRows(c.Bus, c.Addr, rows)
}
