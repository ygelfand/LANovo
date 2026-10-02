//go:build !linux

package ble

import (
	"errors"
	"time"
)

// The chip is on a tty on the device and nowhere else. The parsing and the segmenting are the part
// worth having off it, and those build anywhere.
var errDevice = errors.New("ble: the radio is only on the device")

// Port is the chip's UART.
type Port struct{}

func Open(string) (*Port, error)     { return nil, errDevice }
func OpenNode(string) (*Port, error) { return nil, errDevice }

func (p *Port) Speed(uint32) error        { return errDevice }
func (p *Port) Flow(bool) error           { return errDevice }
func (p *Port) Drain(time.Duration) error { return errDevice }
func (p *Port) Flush() error              { return errDevice }
func (p *Port) Close() error              { return nil }

func (p *Port) Write([]byte) (int, error) { return 0, errDevice }
func (p *Port) Send([]byte) (int, error)  { return 0, errDevice }
func (p *Port) Wake() error               { return errDevice }

func (p *Port) Ask([]byte, time.Duration) (event, error) { return event{}, errDevice }
func (p *Port) Read(time.Duration) (event, error)        { return event{}, errDevice }
func (p *Port) Raw(time.Duration) ([]byte, error)        { return nil, errDevice }
func (p *Port) Sniff(time.Duration) []string             { return nil }
func (p *Port) Asleep() bool                             { return false }
func (p *Port) next(time.Duration) (packet, error)       { return packet{}, errDevice }
func (p *Port) Settle(time.Duration)                     {}

const (
	slow = 0
	fast = 0
)
