package clock

import (
	"context"
	"log/slog"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
)

func (c *Clock) Handle(ctx context.Context, conn *esphome.Conn, msg proto.Message) error {
	if reply, ok := msg.(*api.GetTimeResponse); ok {
		c.fromHome(reply.GetEpochSeconds())
		c.zoneFromHome(reply.GetTimezone())
		return nil
	}

	if c.shouldAsk(conn) {
		c.request(conn)
	}
	return nil
}

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

func (c *Clock) request(conn *esphome.Conn) {
	if conn == nil {
		return
	}
	if err := conn.Send(&api.GetTimeRequest{}); err != nil {
		slog.Debug("asking Home Assistant the time failed", "err", err)
	}
}

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
