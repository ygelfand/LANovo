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
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Device, Get, component.Order(90),
		component.Supervise(service.Restart(time.Second, time.Minute)))
}

// Socket is the abstract address, which the leading NUL is what makes abstract.
const Socket = "@lanovod"

// step is how long a synthesized drag waits between moves. Roughly a touch report, so the device
// sees something close to what a finger produces.
const step = 8 * time.Millisecond

type Control struct {
	mu sync.Mutex

	// id is the tracking id of the contact being synthesized, so each press is a new one.
	id int

	enable *esphome.Switch

	// gate guards the listener, which the switch and the component's own shutdown both reach.
	gate     sync.Mutex
	listener net.Listener

	// addr is where it listens. Socket everywhere real; a test points it at a path of its own,
	// because the name is a fixed one and two tests binding it at once is a race with the device.
	addr string
}

var (
	once   sync.Once
	shared *Control
)

func Get() *Control { once.Do(func() { shared = build() }); return shared }

func build() *Control {
	c := &Control{addr: Socket}

	c.enable = &esphome.Switch{
		Base: esphome.Base{
			ObjectID: "control_socket",
			Name:     "Control socket",
			Icon:     "mdi:console-network",
			Category: esphome.CategoryConfig,
		},
	}
	c.enable.Set(Enabled())
	c.enable.OnCommand = c.SetControl

	return c
}

func (c *Control) Name() string { return "control" }

func (c *Control) Entities() []esphome.Entity { return []esphome.Entity{c.enable} }

// Enabled reports whether the socket is wanted. Off unless somebody has asked for it.
func Enabled() bool { return config.Get().Access.Control }

// SetControl opens the socket or closes it. Home Assistant's switch comes here.
func (c *Control) SetControl(on bool) {
	c.enable.Set(on)

	if err := config.Set().Access().Control(on); err != nil {
		slog.Error("saving the control setting failed", "err", err)
		return
	}

	if !on {
		c.down()
		return
	}
	if err := c.up(); err != nil {
		slog.Error("the control socket would not open", "err", err)
	}
}

// Run holds the socket for as long as it is wanted, and nothing at all when it is not.
func (c *Control) Run(ctx context.Context) error {
	if Enabled() {
		if err := c.up(); err != nil {
			return err
		}
	} else {
		slog.Info("the control socket is closed", "address", Socket)
	}

	<-ctx.Done()
	c.down()
	return nil
}

// up opens the socket, if it is not open already.
func (c *Control) up() error {
	c.gate.Lock()
	defer c.gate.Unlock()

	if c.listener != nil {
		return nil
	}

	l, err := net.Listen("unix", c.addr)
	if err != nil {
		return fmt.Errorf("control: %w", err)
	}
	c.listener = l

	// Loudly, because it is a way in. Anything on the device that can open an abstract socket can
	// drive the screen and read it back, with no pairing and no record of who did.
	slog.Warn("the control socket is open and unauthenticated", "address", c.addr)

	go c.accept(l)
	return nil
}

// down closes it, so nothing is left holding the address.
func (c *Control) down() {
	c.gate.Lock()
	defer c.gate.Unlock()

	if c.listener == nil {
		return
	}
	c.listener.Close()
	c.listener = nil

	slog.Info("the control socket is closed", "address", c.addr)
}

// accept takes callers until the listener is closed, which is the only way it ends.
func (c *Control) accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go c.serve(conn)
	}
}

func (c *Control) serve(conn net.Conn) {
	defer conn.Close()

	in := bufio.NewScanner(conn)
	for in.Scan() {
		out, err := c.run(strings.Fields(in.Text()))

		// Before the error, not instead of it. A command that got several steps in before failing
		// has already said what worked, and that is the part worth reading.
		if out != "" {
			fmt.Fprintln(conn, strings.TrimRight(out, "\n"))
		}
		if err != nil {
			fmt.Fprintf(conn, "error: %v\n", err)
			continue
		}
		fmt.Fprintln(conn, "ok")
	}
}

// run does one command, through the tree in tree.go.
//
// Output is collected rather than printed, because the caller is a socket and not a terminal: what
// the command wrote and what it failed with go back over the same connection in the right order.
func (c *Control) run(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}

	var out strings.Builder

	root := c.tree()
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&out)

	err := root.Execute()
	return out.String(), err
}

func (c *Control) tap(x, y int) {
	id := c.next()
	at := time.Now()

	touch.Get().Deliver(touch.Contact{ID: id, X: x, Y: y, Phase: touch.Down, At: at})
	touch.Get().Deliver(touch.Contact{ID: id, X: x, Y: y, Phase: touch.Up, At: at.Add(step)})
}

// press holds a finger on one point, then lifts it. Move reports in between, because a press that
// only says down and up looks to anything watching like a finger that vanished.
func (c *Control) press(x, y int, hold time.Duration) error {
	id := c.next()
	touch.Get().Deliver(touch.Contact{ID: id, X: x, Y: y, Phase: touch.Down, At: time.Now()})

	for left := hold; left > 0; left -= step {
		time.Sleep(min(left, step))
		touch.Get().Deliver(touch.Contact{ID: id, X: x, Y: y, Phase: touch.Move, At: time.Now()})
	}

	touch.Get().Deliver(touch.Contact{ID: id, X: x, Y: y, Phase: touch.Up, At: time.Now()})
	return nil
}

func (c *Control) swipe(args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("want X1 Y1 X2 Y2 [steps]")
	}

	fromX, fromY, err := point(args[0:2])
	if err != nil {
		return err
	}

	toX, toY, err := point(args[2:4])
	if err != nil {
		return err
	}

	steps := 20
	if len(args) > 4 {
		if steps, err = strconv.Atoi(args[4]); err != nil || steps < 1 {
			return fmt.Errorf("steps must be a positive number")
		}
	}

	id := c.next()
	now := time.Now()

	touch.Get().Deliver(touch.Contact{ID: id, X: fromX, Y: fromY, Phase: touch.Down, At: now})

	for i := 1; i <= steps; i++ {
		time.Sleep(step)

		x := fromX + (toX-fromX)*i/steps
		y := fromY + (toY-fromY)*i/steps
		touch.Get().Deliver(touch.Contact{ID: id, X: x, Y: y, Phase: touch.Move, At: time.Now()})
	}

	touch.Get().Deliver(touch.Contact{ID: id, X: toX, Y: toY, Phase: touch.Up, At: time.Now()})
	return nil
}

// next is a tracking id nothing else is using. A press that reused one would be matched to the
// journey of the press before it.
func (c *Control) next() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.id++
	return c.id
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

func point(args []string) (x, y int, err error) {
	if len(args) < 2 {
		return 0, 0, fmt.Errorf("want X Y")
	}

	if x, err = strconv.Atoi(args[0]); err != nil {
		return 0, 0, fmt.Errorf("x: %w", err)
	}
	if y, err = strconv.Atoi(args[1]); err != nil {
		return 0, 0, fmt.Errorf("y: %w", err)
	}
	return x, y, nil
}

// picture is the screen as it is being viewed.
func picture() (w, h int) {
	return display.Get().Orientation().Size(board.Current().PanelWidth, board.Current().PanelHeight)
}
