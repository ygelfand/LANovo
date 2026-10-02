package clock

import (
	"context"
	"log/slog"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
)

// Home Assistant knows the time and is already talking to this device, so it is asked first: it
// works on a network with no route out, which a time server does not, and it is the same source
// an ESPHome device with `time: platform: homeassistant` uses.
//
// The protocol has the device ask and the client answer, so this both sends the request and takes
// the reply.

// Handle takes the answer, and asks the question the first time a client says anything.
func (c *Clock) Handle(ctx context.Context, conn *esphome.Conn, msg proto.Message) error {
	if reply, ok := msg.(*api.GetTimeResponse); ok {
		c.fromHome(reply.GetEpochSeconds())
		c.zoneFromHome(reply.GetTimezone())
		return nil
	}

	// Any message means a client is there to ask. Once per connection is enough: the clock is
	// checked again on its own timer afterwards, and a client that reconnects is asked afresh.
	if c.shouldAsk(conn) {
		c.request(conn)
	}
	return nil
}

// shouldAsk reports whether this connection has not been asked the time yet, and records that it
// is about to be.
//
// Per connection rather than once for the life of the process: a client that went away and came
// back is one worth asking again, and after a long disconnection it is the better source.
func (c *Clock) shouldAsk(conn *esphome.Conn) bool {
	if conn == nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if conn == c.asked {
		return false
	}
	c.asked = conn
	return true
}

// Ask sends the request on a connection.
func (c *Clock) request(conn *esphome.Conn) {
	if conn == nil {
		return
	}
	if err := conn.Send(&api.GetTimeRequest{}); err != nil {
		slog.Debug("asking Home Assistant the time failed", "err", err)
	}
}

// fromHome takes an epoch Home Assistant reported.
func (c *Clock) fromHome(epoch uint32) {
	if epoch == 0 {
		return
	}

	at := time.Unix(int64(epoch), 0)
	offset := time.Until(at)

	if err := c.accept(offset, "home assistant"); err != nil {
		slog.Warn("could not take the time from Home Assistant", "err", err)
	}
}
