package control

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(90),
		sharedcomponent.Supervise(service.Restart(time.Second, time.Minute)))
}

// A leading NUL makes a unix socket address abstract.
const Socket = "@lanovod"

type Control struct {
	*harness.Server

	inputOnce sync.Once
	input     *harness.Input
}

var Get = sync.OnceValue(func() *Control { return build(Socket) })

func build(addr string) *Control {
	c := &Control{}
	c.Server = harness.NewServer(addr, harness.Executes(c.tree))
	return c
}

func (c *Control) Name() string { return "control" }

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
