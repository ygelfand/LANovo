package bluetooth

import (
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/ble"
)

// The reader must never wait on the network. It runs on the goroutine draining the controller, so a
// queue that blocked when Home Assistant fell behind would stall that reader, overflow the
// controller's own queue, and make its driver discard packets for everything on the chip — wifi,
// which is the only way to reach this device, included.
//
// Timed rather than counted, because the failure is a goroutine that never returns and a test that
// only counted drops would hang instead of failing.
func TestQueueNeverWaitsForHomeAssistant(t *testing.T) {
	b := &Proxy{reports: make(chan ble.Advertisement, 2)}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			b.queue(ble.Advertisement{})
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queueing blocked with nobody reading, which would stall the controller")
	}

	if got := b.Dropped(); got != 98 {
		t.Errorf("%d reports dropped past a queue of 2, want 98", got)
	}
}

// What does fit is kept, in order. A queue that dropped the oldest instead would report the wrong
// RSSI for a device that is moving, which is the case the proxy exists to serve.
func TestQueueKeepsWhatFits(t *testing.T) {
	b := &Proxy{reports: make(chan ble.Advertisement, 4)}

	for i := range 4 {
		b.queue(ble.Advertisement{RSSI: int8(-i)})
	}

	for i := range 4 {
		select {
		case a := <-b.reports:
			if a.RSSI != int8(-i) {
				t.Errorf("report %d has rssi %d, want %d", i, a.RSSI, -i)
			}
		default:
			t.Fatalf("only %d reports queued, want 4", i)
		}
	}

	if got := b.Dropped(); got != 0 {
		t.Errorf("%d dropped with room for every report, want 0", got)
	}
}
