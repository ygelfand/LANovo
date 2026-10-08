package control

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Device, Get, component.Order(90),
		component.Supervise(service.Restart(time.Second, time.Minute)))
}

// A leading NUL makes a unix socket address abstract.
const Socket = "@lanovod"

type Control struct {
	inputOnce sync.Once
	input     *harness.Input

	gate    sync.Mutex
	runtime *harness.Server

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

func (c *Control) server() *harness.Server {
	c.gate.Lock()
	defer c.gate.Unlock()
	if c.runtime == nil {
		c.runtime = harness.NewServer(
			c.addr,
			func(ctx context.Context, args []string) (string, error) { return harness.Execute(ctx, c.tree, args) },
		)
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
	harness.Serve(
		context.Background(),
		conn,
		func(ctx context.Context, args []string) (string, error) { return harness.Execute(ctx, c.tree, args) },
	)
}
func (c *Control) run(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	return harness.Execute(context.Background(), c.tree, args)
}

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

func picture() (w, h int) {
	return display.Get().Orientation().Size(board.Current().PanelWidth, board.Current().PanelHeight)
}
