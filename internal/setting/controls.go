package setting

import (
	"log/slog"
	"strconv"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"
)

// A table rendered as Home Assistant entities: a Switch for a Toggle, a Number for a Number, a
// Select for a Choice. A kind with no shape Home Assistant understands is left out rather than
// approximated.

// Controls builds and holds those entities. Want picks which groups are offered; nil is all of them.
type Controls[T any] struct {
	Table  *Table[T]
	Device uint32
	Want   func(Group) bool

	// Read is the settings as they stand, and Save makes one stick.
	//
	// Save takes the row rather than its name: a feature whose settings are a map can write the
	// name, and one whose settings are typed fields hands the row the struct to write itself.
	Read func() T
	Save func(s Setting[T], value string) error

	once  sync.Once
	built []esphome.Entity
	rows  []Setting[T]
}

func (c *Controls[T]) Entities() []esphome.Entity {
	c.once.Do(c.build)
	return c.built
}

// Entity is the control one row was rendered as. The feature that owns the table is the only thing
// that knows a row by name, and it is what reads back what Home Assistant was told.
func (c *Controls[T]) Entity(name string) esphome.Entity {
	c.once.Do(c.build)

	for i, s := range c.rows {
		if s.Name == name {
			return c.built[i]
		}
	}
	return nil
}

// Publish reads the settings back out to every entity.
//
// All of them, not the one that was set: one setting can move another, and a named white balance
// moving both gain knobs is exactly that.
func (c *Controls[T]) Publish() { c.PublishFrom(c.Read()) }

// PublishFrom says what the settings are, for a caller holding a value the store does not have yet
// — one being restored, or one applied without being written.
func (c *Controls[T]) PublishFrom(at T) {
	c.once.Do(c.build)

	for i, s := range c.rows {
		switch e := c.built[i].(type) {
		case *esphome.Switch:
			e.Set(s.On(&at))
		case *esphome.Number:
			e.Set(float32(s.Level(&at)))
		case *esphome.Select:
			e.Set(s.LabelOf(s.Read(&at)))
		}
	}
}

func (c *Controls[T]) build() {
	at := c.Read()

	for _, g := range c.Table.Groups() {
		if c.Want != nil && !c.Want(g) {
			continue
		}
		for _, s := range c.Table.In(g) {
			e := c.entity(s, &at)
			if e == nil {
				continue
			}
			c.built = append(c.built, e)
			c.rows = append(c.rows, s)
		}
	}
}

// esphome.Base carries a mutex, so every one of these is built in place rather than copied in.
func (c *Controls[T]) entity(s Setting[T], at *T) esphome.Entity {
	id, name, icon, device := s.Object(c.Table.Domain), s.Named(), s.Icon, c.Device

	switch s.Kind {
	case Toggle:
		e := &esphome.Switch{Base: esphome.Base{
			ObjectID: id, DeviceID: device, Name: name, Icon: icon,
			Category: esphome.CategoryConfig,
		}}
		e.Set(s.On(at))
		e.OnCommand = func(on bool) { c.put(s, OnOff(on)) }
		return e

	case Number:
		mode := esphome.NumberBox
		if s.Slider {
			mode = esphome.NumberSlider
		}
		e := &esphome.Number{
			Base: esphome.Base{
				ObjectID: id, DeviceID: device, Name: name, Icon: icon,
				Category: esphome.CategoryConfig,
			},
			Min: float64(s.Min), Max: float64(s.Max), Step: 1,
			Unit: s.Unit,
			Mode: mode,
		}
		e.Set(float32(s.Level(at)))
		e.OnCommand = func(v float32) { c.put(s, strconv.Itoa(int(v))) }
		return e

	case Choice:
		e := &esphome.Select{
			Base: esphome.Base{
				ObjectID: id, DeviceID: device, Name: name, Icon: icon,
				Category: esphome.CategoryConfig,
			},
			Options: s.Labels(),
		}
		e.Set(s.LabelOf(s.Read(at)))
		e.OnCommand = func(label string) { c.put(s, s.ValueOf(label)) }
		return e
	}
	return nil
}

func (c *Controls[T]) put(s Setting[T], v string) {
	if v == "" {
		return
	}
	if err := c.Save(s, v); err != nil {
		slog.Error("the setting could not be saved", "setting", s.Name, "err", err)
		return
	}
	c.Publish()
}
