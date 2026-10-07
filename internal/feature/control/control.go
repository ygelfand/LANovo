// Package control is a way to drive the device without touching it.
//
// A line-oriented socket that synthesizes touches and reports what the device is showing, so the
// screen can be exercised from a terminal — a drag measured under a profiler, a screen opened and
// checked, a sequence replayed the same way twice. None of that is possible with a finger.
//
// Local only: an abstract unix socket, which has no filesystem presence and cannot be reached off
// the device.
package control

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/service"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
)

func init() {
	component.Register(component.Device, Get, component.Order(90),
		component.Supervise(service.Restart(time.Second, time.Minute)))
}

// Socket is the abstract address, which the leading NUL is what makes abstract.
const Socket = "@lanovod"

type Control struct {
	inputOnce sync.Once
	input     *harness.Input

	// gate guards initialization of the shared server, including the host-test address.
	gate    sync.Mutex
	runtime *harness.Server

	// addr is where it listens. Socket everywhere real; a test points it at a path of its own,
	// because the name is a fixed one and two tests binding it at once is a race with the device.
	addr string
}

var (
	once   sync.Once
	shared *Control
)

func Get() *Control { once.Do(func() { shared = build() }); return shared }

func build() *Control           { return &Control{addr: Socket} }
func (c *Control) Name() string { return "control" }
func (c *Control) Run(ctx context.Context) error {
	if err := c.up(); err != nil {
		return err
	}
	<-ctx.Done()
	c.down()
	return nil
}

// server is created lazily so host tests can choose their own address.
func (c *Control) server() *harness.Server {
	c.gate.Lock()
	defer c.gate.Unlock()
	if c.runtime == nil {
		c.runtime = harness.NewServer(c.addr, func(ctx context.Context, args []string) (string, error) { return harness.Execute(ctx, c.tree, args) })
	}
	return c.runtime
}
func (c *Control) up() error { return c.server().Start() }
func (c *Control) down() {
	c.gate.Lock()
	server := c.runtime
	c.gate.Unlock()
	if server != nil {
		server.Close()
	}
}
func (c *Control) serve(conn net.Conn) {
	harness.Serve(context.Background(), conn, func(ctx context.Context, args []string) (string, error) { return harness.Execute(ctx, c.tree, args) })
}
func (c *Control) run(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	return harness.Execute(context.Background(), c.tree, args)
}

// level sets one stream's volume, which is the only way to bring the card up without a hand on the
// buttons on the side of the device. Set rather than Adjust, so checking a screen makes no sound.
func level(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("want STREAM LEVEL")
	}

	stream := config.Stream(args[0])
	if !slices.Contains(config.Streams(), stream) {
		return fmt.Errorf("no such stream %q", args[0])
	}

	at, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("level: %w", err)
	}

	volume.Get().Set(stream, at)
	return nil
}

// picture is the screen as it is being viewed.
func picture() (w, h int) {
	return display.Get().Orientation().Size(board.Current().PanelWidth, board.Current().PanelHeight)
}
