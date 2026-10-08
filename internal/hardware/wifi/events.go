package wifi

import (
	"context"
	"time"

	"github.com/ygelfand/libcountertop/pkg/network/wpa"
)

const (
	EventConnected    = wpa.EventConnected
	EventStateChange  = wpa.EventStateChange
	EventDisconnected = wpa.EventDisconnected
	EventTerminating  = wpa.EventTerminating
)

type State = wpa.StateChange
type Event = wpa.Event

var state = wpa.ParseState

type eventSocket struct{ *Control }

func (c eventSocket) Read(buf []byte) (int, error)       { return c.conn.Read(buf) }
func (c eventSocket) SetReadDeadline(at time.Time) error { return c.conn.SetReadDeadline(at) }

func Listen(ctx context.Context, handler func(Event)) error { return listen(ctx, nil, handler) }
func listen(ctx context.Context, initial func(), handler func(Event)) error {
	c, err := Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	return wpa.Monitor(ctx, eventSocket{c}, initial, handler)
}
