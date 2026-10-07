package detect

import (
	"github.com/ygelfand/LANovo/internal/config"
	esphome "github.com/ygelfand/go-esphome-device"
	sharedengine "github.com/ygelfand/libcountertop/pkg/inference/detect"
)

const StopSlot = sharedengine.StopSlot

func (d *Detect) Entities() []esphome.Entity { return []esphome.Entity{d.stop.Entity} }
func (d *Detect) Restore(c config.Config)    { d.stop.Restore(c.Wake.Stop.Threshold) }
