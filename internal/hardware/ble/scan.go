package ble

import (
	"context"
	"encoding/binary"
	"fmt"
)

// The LE commands a scanner sends.
const (
	hciLEScanParams = 0x200b
	hciLEScanEnable = 0x200c
)

// What an LE Meta event can be. Only one of them is a scanner's business.
const (
	eventLEMeta = 0x3e

	leAdvertisingReport = 0x02
)

// How much of the time the radio listens, in units of 0.625ms: 18.75ms out of every 200ms. Wifi is
// on the same chip and antenna, and a window equal to the interval never gives it back.
const (
	scanInterval = 320
	scanWindow   = 30
)

// Advertisement is one LE advertising report.
type Advertisement struct {
	Address     [6]byte
	AddressType byte
	RSSI        int8
	Data        []byte
}

// Addr is the address as a big-endian integer, which is how Home Assistant wants it.
func (a Advertisement) Addr() uint64 {
	var v uint64
	for _, b := range a.Address {
		v = v<<8 | uint64(b)
	}
	return v
}

// Scan turns on LE scanning and calls found for every advertisement until the context ends. Active
// asks for scan responses, which transmits rather than only listening.
func Scan(ctx context.Context, s *Session, active bool, found func(Advertisement)) error {
	// Registered before the scan is turned on, or the first advertisements arrive with nowhere to
	// go and the beacons that report slowest are the ones missed.
	got, done := s.Reports()
	defer done()

	if err := scanning(s, active, true); err != nil {
		return err
	}

	// Even on the way out of an error: a controller left scanning keeps the antenna busy.
	defer scanning(s, active, false)

	for {
		select {
		case <-ctx.Done():
			return nil
		case g, ok := <-got:
			if !ok {
				return fmt.Errorf("ble: scanning: %w", s.Err())
			}
			reports(g.event.Params, found)
		}
	}
}

// scanning turns the scan on or off.
func scanning(s *Session, active, on bool) error {
	if on {
		params := make([]byte, 7)
		if active {
			params[0] = 0x01
		}
		binary.LittleEndian.PutUint16(params[1:], scanInterval)
		binary.LittleEndian.PutUint16(params[3:], scanWindow)

		if _, err := s.Command(hciLEScanParams, params...); err != nil {
			return fmt.Errorf("ble: the scan parameters: %w", err)
		}
	}

	// Enable, and whether to filter duplicates. Kept: how often a beacon repeats is Home
	// Assistant's to judge.
	enable := byte(0x00)
	if on {
		enable = 0x01
	}
	if _, err := s.Command(hciLEScanEnable, enable, 0x00); err != nil {
		return fmt.Errorf("ble: turning the scan %v: %w", on, err)
	}
	return nil
}

// reports walks an LE Meta event: event type, address type, address, data length, data, RSSI.
//
// A malformed one is dropped rather than guessed at: this is the only place on the device parsing
// something a stranger in the street can send.
func reports(p []byte, found func(Advertisement)) {
	if len(p) < 2 || p[0] != leAdvertisingReport {
		return
	}

	at := 2
	for range int(p[1]) {
		if at+9 > len(p) {
			return
		}

		length := int(p[at+8])
		end := at + 9 + length
		if end >= len(p) {
			return
		}

		a := Advertisement{AddressType: p[at+1], RSSI: int8(p[end])}

		// HCI carries the address least significant octet first; Address is the MAC as written,
		// which is the order a resolvable private address must be in to match an identity key.
		for i := range a.Address {
			a.Address[i] = p[at+7-i]
		}
		a.Data = append([]byte(nil), p[at+9:end]...)
		found(a)

		at = end + 1
	}
}
