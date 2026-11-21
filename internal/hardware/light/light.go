package light

import (
	"errors"
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/system/i2c"
)

type Rises [5]float64

type Sensor interface {
	Lux() (float64, error)
	Proximity() (int, error)
	Rises() Rises
}

var (
	_ Sensor = (*VCNL4200)(nil)
	_ Sensor = (*LTR578)(nil)
)

func Open() (Sensor, i2c.Bus, error) {
	s, bus, err := open(vcnl4200Buses, vcnl4200Address, isVCNL4200,
		func(d *i2c.Device) (Sensor, error) { return AttachVCNL4200(d) })
	if err == nil {
		return s, bus, nil
	}
	s, bus, lerr := open(ltr578Buses, ltr578Address, isLTR578,
		func(d *i2c.Device) (Sensor, error) { return AttachLTR578(d) })
	if lerr == nil {
		return s, bus, nil
	}
	return nil, nil, errors.Join(err, lerr)
}

func open(
	buses []int,
	addr uint16,
	identify func(*i2c.Device) bool,
	attach func(*i2c.Device) (Sensor, error),
) (Sensor, i2c.Bus, error) {
	dev, bus, err := i2c.Find(buses, addr, identify)
	if err != nil {
		return nil, nil, fmt.Errorf("light: %w", err)
	}
	s, err := attach(dev)
	if err != nil {
		_ = bus.Close()
		return nil, nil, err
	}
	return s, bus, nil
}
