package clock

import (
	"testing"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
)

func TestWorthSetting(t *testing.T) {
	tests := []struct {
		name   string
		offset time.Duration
		want   bool
	}{
		{"already right", 0, false},
		{"a little fast", 200 * time.Millisecond, false},
		{"a little slow", -200 * time.Millisecond, false},
		{"exactly the threshold is not past it", step, false},
		{"seconds out", 5 * time.Second, true},
		{"seconds out the other way", -5 * time.Second, true},
		{"a boot in 1970", 56 * 365 * 24 * time.Hour, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worthSetting(tt.offset); got != tt.want {
				t.Errorf("worthSetting(%v) = %v, want %v", tt.offset, got, tt.want)
			}
		})
	}
}

// With no lease there is nowhere else to ask, and a device with no time is worse than one asking
// the pool.
func TestServersFallBackToThePool(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}

	got := c.servers()
	if len(got) != len(pool) {
		t.Fatalf("servers() = %v, want the pool %v", got, pool)
	}
	for i := range got {
		if got[i] != pool[i] {
			t.Errorf("server %d is %q, want %q", i, got[i], pool[i])
		}
	}
}

func TestReadyClosesOnce(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}

	select {
	case <-c.Ready():
		t.Fatal("Ready is closed before the first sync")
	default:
	}

	// An offset small enough not to touch the clock, which a test may not do anyway.
	for range 2 {
		if err := c.accept(time.Millisecond, "test"); err != nil {
			t.Fatalf("accept: %v", err)
		}
	}

	select {
	case <-c.Ready():
	default:
		t.Fatal("Ready is open after a sync")
	}
}

func TestOffsetIsRecorded(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}

	if err := c.accept(250*time.Millisecond, "test"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	offset, at := c.Offset()
	if offset != 250*time.Millisecond {
		t.Errorf("offset = %v, want 250ms", offset)
	}
	if at.IsZero() {
		t.Error("the time of the last sync was not recorded")
	}
}

func TestSyncedFires(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}

	fired := make(chan time.Time, 1)
	cancel := c.Synced.Listen(func(at time.Time) { fired <- at })
	defer cancel()

	if err := c.accept(time.Millisecond, "test"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("Synced did not fire")
	}
}

// Home Assistant is asked once per connection, not once per message: every message from a client
// passes through Handle, and asking on each would be a request per message.
func TestAskedOncePerConnection(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}
	conn := new(esphome.Conn)

	if !c.shouldAsk(conn) {
		t.Fatal("the first message on a connection did not ask the time")
	}
	for range 5 {
		if c.shouldAsk(conn) {
			t.Fatal("a later message on the same connection asked again")
		}
	}
}

// A client that reconnects is asked afresh. The clock had been asked once for the life of the
// process, so a device that ran for weeks never took the time from Home Assistant again.
func TestAskedAgainOnANewConnection(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}

	if !c.shouldAsk(new(esphome.Conn)) {
		t.Fatal("the first connection was not asked")
	}
	if !c.shouldAsk(new(esphome.Conn)) {
		t.Error("a reconnecting client was never asked the time again")
	}
}

// Nothing to ask is not something to ask.
func TestNoConnectionIsNotAsked(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}

	if c.shouldAsk(nil) {
		t.Error("the time was asked of nothing")
	}
}

// The clock is set once and then drifts, so a sync that has already succeeded is still worth
// making. accept is what a round ends in, and it has to keep taking new offsets rather than
// settling on the first.
func TestSyncingAgainAfterTheClockIsSet(t *testing.T) {
	c := &Clock{ready: make(chan struct{})}

	if err := c.accept(time.Millisecond, "first"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if !c.Set() {
		t.Fatal("the clock does not report itself set after a sync")
	}

	if err := c.accept(5*time.Millisecond, "second"); err != nil {
		t.Fatalf("accept after the clock was set: %v", err)
	}

	if offset, _ := c.Offset(); offset != 5*time.Millisecond {
		t.Errorf("offset = %v, want the newer 5ms: a set clock stopped taking new offsets", offset)
	}
}
