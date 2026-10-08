package display

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/hook"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(5),
		component.Supervise(service.Restart(time.Second, 30*time.Second)))
}

type Priority int

const (
	PriorityDashboard Priority = iota

	PrioritySetup

	PriorityUI

	PriorityNotice

	PriorityAlert

	PriorityBoot
)

type Driver struct {
	path string

	damaged Rect

	forced bool

	dropped error

	opened   bool
	stranded func(*Panel) error
	unstrand func()

	parting bool

	frames, whole int
	painted       int64

	Released hook.Hook[Priority]

	mu     sync.Mutex
	panel  *Panel
	claims []*Claim

	info string

	brightness int

	rot Orientation

	changed chan struct{}

	shots chan chan capture

	lasts chan lastFrame

	benches chan benchRequest
}

type benchRequest struct {
	frames int

	stop time.Time

	done chan benchResult
}

type benchResult struct {
	bench Bench
	err   error
}

type Bench struct {
	Frames    int
	Draw, Pan time.Duration
	Width     int
	Height    int
}

func (b Bench) Each() (draw, pan time.Duration) {
	if b.Frames == 0 {
		return 0, 0
	}
	return b.Draw / time.Duration(b.Frames), b.Pan / time.Duration(b.Frames)
}

func (b Bench) Rate() float64 {
	total := b.Draw + b.Pan
	if b.Frames == 0 || total == 0 {
		return 0
	}
	return float64(b.Frames) / total.Seconds()
}

func (b Bench) String() string {
	draw, pan := b.Each()
	return fmt.Sprintf("frames %d %dx%d draw %.2fms pan %.2fms frame %.2fms %.1f fps",
		b.Frames, b.Width, b.Height,
		float64(draw.Microseconds())/1000, float64(pan.Microseconds())/1000,
		float64((draw+pan).Microseconds())/1000, b.Rate())
}

type lastFrame struct {
	draw func(*Panel) error
	done chan error
}

type capture struct {
	pixels []byte
	w, h   int
	err    error
}

var (
	once   sync.Once
	shared *Driver
)

func Get() *Driver { once.Do(func() { shared = NewDriver(layout.FBDevice) }); return shared }

func NewDriver(path string) *Driver {
	return &Driver{
		path:       path,
		changed:    make(chan struct{}, 1),
		shots:      make(chan chan capture),
		lasts:      make(chan lastFrame),
		benches:    make(chan benchRequest),
		brightness: DefaultBrightness,
		rot:        Mounted(),
	}
}

func (d *Driver) Name() string { return "display" }

func (d *Driver) Startup() component.Progress {
	d.mu.Lock()
	panel := d.panel
	d.mu.Unlock()

	if panel == nil {
		return component.Progress{Doing: "waiting for the screen"}
	}

	return component.Progress{
		Done:  true,
		Doing: fmt.Sprintf("%d×%d", panel.Width, panel.Height),
	}
}

func (d *Driver) Start(ctx context.Context) error {
	p, err := d.open(ctx)
	if err != nil {
		return err
	}

	d.mu.Lock()
	brightness, rot := d.brightness, d.rot
	d.mu.Unlock()

	p.Turn(rot)

	p.Dropped.Listen(d.drop)

	d.mu.Lock()
	d.panel = p
	d.info = p.Info()
	info := d.info
	d.mu.Unlock()

	if err := SetBacklight(brightness); err != nil {
		slog.Error("backlight", "err", err)
	}

	slog.Info("panel", "geometry", info, "brightness", brightness)
	return nil
}

func (d *Driver) Covered(below Priority) bool { return d.top(below) != nil }

func (d *Driver) Uncovered(p Priority) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.claims {
		if c.priority <= p || !c.covers {
			continue
		}
		c.mu.Lock()
		has := c.draw != nil
		c.mu.Unlock()
		if has {
			return false
		}
	}
	return true
}

func (d *Driver) Stats() (frames, whole int, painted int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.frames, d.whole, d.painted
}

var shotWait = 2 * time.Second

func (d *Driver) Shot() (pixels []byte, w, h int, err error) {
	reply := make(chan capture, 1)

	timeout := time.NewTimer(shotWait)
	defer timeout.Stop()

	select {
	case d.shots <- reply:
	case <-timeout.C:
		return nil, 0, 0, fmt.Errorf("display: nothing is rendering to take a screenshot between")
	}

	select {
	case got := <-reply:
		return got.pixels, got.w, got.h, got.err
	case <-timeout.C:
		return nil, 0, 0, fmt.Errorf("display: the screenshot was not answered")
	}
}

func (d *Driver) Orientation() Orientation {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rot
}

func (d *Driver) SetOrientation(rot Orientation) {
	d.mu.Lock()
	if d.rot == rot {
		d.mu.Unlock()
		return
	}
	d.rot = rot
	d.forced = true
	d.mu.Unlock()

	d.wake()
}

func (d *Driver) Brightness(percent int) error {
	d.mu.Lock()
	d.brightness = percent
	open := d.panel != nil
	d.mu.Unlock()

	if !open {
		return nil
	}
	return SetBacklight(percent)
}

func (d *Driver) Run(ctx context.Context) error {
	d.mu.Lock()
	open := d.panel != nil
	d.mu.Unlock()

	if !open {
		return fmt.Errorf("display: no panel")
	}

	for {
		if err := d.render(); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-d.changed:
		case reply := <-d.shots:
			reply <- d.capture()
		case f := <-d.lasts:
			f.done <- d.paint(f.draw)
		case b := <-d.benches:
			bench, err := d.measure(b.frames, b.stop)
			b.done <- benchResult{bench: bench, err: err}
		}
	}
}

func (d *Driver) capture() capture {
	d.mu.Lock()
	panel := d.panel
	d.mu.Unlock()

	if panel == nil {
		return capture{err: fmt.Errorf("display: no panel")}
	}

	pixels, w, h := panel.Snapshot()
	return capture{pixels: pixels, w: w, h: h}
}

func (d *Driver) Last(ctx context.Context, draw func(*Panel) error) error {
	done := make(chan error, 1)

	select {
	case d.lasts <- lastFrame{draw: draw, done: done}:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Driver) paint(draw func(*Panel) error) error {
	d.mu.Lock()
	panel := d.panel
	d.mu.Unlock()

	if panel == nil {
		return fmt.Errorf("display: no panel")
	}
	if err := draw(panel); err != nil {
		return err
	}
	if err := panel.Flip(); err != nil {
		return err
	}

	d.mu.Lock()
	d.parting = true
	d.mu.Unlock()
	return nil
}

func (d *Driver) Bench(ctx context.Context, frames int) (Bench, error) {
	if frames <= 0 {
		return Bench{}, fmt.Errorf("display: ask for at least one frame")
	}
	done := make(chan benchResult, 1)

	stop, ok := ctx.Deadline()
	if !ok {
		stop = time.Now().Add(time.Minute)
	}

	select {
	case d.benches <- benchRequest{frames: frames, stop: stop, done: done}:
	case <-ctx.Done():
		return Bench{}, ctx.Err()
	}

	select {
	case r := <-done:
		return r.bench, r.err
	case <-ctx.Done():
		return Bench{}, ctx.Err()
	}
}

func (d *Driver) measure(frames int, stop time.Time) (Bench, error) {
	d.mu.Lock()
	panel := d.panel
	d.mu.Unlock()

	if panel == nil {
		return Bench{}, fmt.Errorf("display: no panel")
	}

	stack, clear := d.stack()
	if len(stack) == 0 {
		return Bench{}, fmt.Errorf("display: nothing is on screen to draw")
	}

	b := Bench{Width: panel.Width, Height: panel.Height}

	for range frames {
		start := time.Now()
		if !start.Before(stop) {
			break
		}

		if clear {
			panel.Fill(0, 0, 0)
		}
		for _, c := range stack {
			c.mu.Lock()
			draw := c.draw
			c.mu.Unlock()

			if draw == nil {
				continue
			}
			if err := draw(panel); err != nil {
				return b, fmt.Errorf("display: %w", err)
			}
		}

		mid := time.Now()
		if err := panel.Flip(); err != nil {
			return b, err
		}

		b.Draw += mid.Sub(start)
		b.Pan += time.Since(mid)
		b.Frames++
	}
	return b, nil
}

func (d *Driver) Close() error {
	d.mu.Lock()
	p := d.panel
	d.panel = nil
	d.mu.Unlock()

	if p == nil {
		return nil
	}
	return p.Close()
}

func (d *Driver) Claim(p Priority) *Claim { return d.claim(p, true) }

func (d *Driver) Overlay(p Priority) *Claim { return d.claim(p, false) }

func (d *Driver) claim(p Priority, covers bool) *Claim {
	c := &Claim{driver: d, priority: p, covers: covers}

	d.mu.Lock()
	d.claims = append(d.claims, c)
	d.forced = true
	d.mu.Unlock()

	d.wake()
	return c
}

type Claim struct {
	driver   *Driver
	priority Priority

	covers bool

	mu       sync.Mutex
	draw     func(*Panel) error
	released bool

	dirty  bool
	damage Rect
}

func (c *Claim) Show(draw func(*Panel) error) {
	c.mu.Lock()
	c.draw = draw

	c.damage, c.dirty = Rect{}, true

	released := c.released
	c.mu.Unlock()

	if !released {
		c.driver.wake()
	}
}

func (c *Claim) ShowIn(damage Rect, draw func(*Panel) error) {
	c.mu.Lock()
	c.draw = draw
	c.grow(damage)

	released := c.released
	c.mu.Unlock()

	if !released {
		c.driver.wake()
	}
}

func (c *Claim) Clear() {
	c.Show(nil)
	d := c.driver
	d.mu.Lock()
	d.forced = true
	d.mu.Unlock()
	d.wake()
}

func (c *Claim) Release() {
	c.mu.Lock()
	if c.released {
		c.mu.Unlock()
		return
	}
	c.released = true
	c.mu.Unlock()

	d := c.driver
	d.mu.Lock()
	for i, held := range d.claims {
		if held == c {
			d.claims = append(d.claims[:i], d.claims[i+1:]...)
			break
		}
	}
	d.forced = true
	d.mu.Unlock()

	d.wake()
	d.Released.Emit(c.priority)
}

func (d *Driver) Held(p Priority) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.claims {
		if c.priority == p {
			return true
		}
	}
	return false
}

func (d *Driver) topmost() *Claim { return d.top(anyPriority) }

func (d *Driver) Showing() (Priority, bool) {
	top := d.topmost()
	if top == nil {
		return 0, false
	}
	return top.priority, true
}

func (d *Driver) stack() (draw []*Claim, clear bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	showing := make([]*Claim, 0, len(d.claims))
	for _, c := range d.claims {
		c.mu.Lock()
		has := c.draw != nil
		c.mu.Unlock()

		if has {
			showing = append(showing, c)
		}
	}

	slices.SortStableFunc(
		showing,
		func(a, b *Claim) int { return cmp.Compare(a.priority, b.priority) },
	)

	for i := len(showing) - 1; i >= 0; i-- {
		if showing[i].covers {
			return showing[i:], false
		}
	}
	return showing, true
}

const anyPriority = PriorityBoot + 1

func (d *Driver) top(ceiling Priority) *Claim {
	d.mu.Lock()
	defer d.mu.Unlock()

	var top *Claim
	for _, c := range d.claims {
		if c.priority >= ceiling {
			continue
		}
		if top != nil && c.priority < top.priority {
			continue
		}

		c.mu.Lock()
		has := c.draw != nil
		c.mu.Unlock()

		if has {
			top = c
		}
	}
	return top
}

func (d *Driver) Native() (w, h int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.panel == nil {
		return 0, 0
	}
	return d.panel.Native()
}

func (d *Driver) drop(err error) {
	d.mu.Lock()
	d.dropped = err
	d.mu.Unlock()
	d.wake()
}

func (d *Driver) wake() {
	select {
	case d.changed <- struct{}{}:
	default:
	}
}

func (d *Driver) render() error {
	d.mu.Lock()
	panel, rot, parting, dropped := d.panel, d.rot, d.parting, d.dropped
	d.dropped = nil
	d.mu.Unlock()

	if parting {
		return nil
	}

	if dropped != nil && panel != nil && panel.surf != nil && panel.surf.Err() != nil {
		slog.Warn("display: lost the helper", "err", dropped)
		if err := panel.redial(dropped); err != nil {
			return err
		}
		slog.Info("display: helper back")
		d.mu.Lock()
		d.forced = true
		d.mu.Unlock()
	}

	stack, clear := d.stack()
	if panel == nil {
		return nil
	}
	if len(stack) == 0 {
		d.mu.Lock()
		forced := d.forced
		d.forced = false
		d.mu.Unlock()
		if !forced {
			return nil
		}
		for range 2 {
			panel.ClearRect(0, 0, panel.Width, panel.Height)
			if err := panel.Flip(); err != nil {
				return err
			}
		}
		return nil
	}

	if panel.Orientation() != rot {
		panel.Turn(rot)

		d.mu.Lock()
		d.info = panel.Info()
		d.mu.Unlock()
	}

	now, any := damageOf(stack)

	d.mu.Lock()
	forced := d.forced
	d.forced = false
	d.mu.Unlock()

	if !any && !forced {
		return nil
	}

	if forced {
		now = Rect{}
	}

	region := union(now, d.damaged)
	d.damaged = now

	panel.Clip(region)
	defer panel.Clip(Rect{})

	d.mu.Lock()
	d.frames++
	if region.W <= 0 || region.H <= 0 {
		d.whole++
		d.painted += int64(panel.Width) * int64(panel.Height)
	} else {
		d.painted += int64(region.W) * int64(region.H)
	}
	d.mu.Unlock()

	if clear {
		panel.Fill(0, 0, 0)
	}

	for _, c := range stack {
		c.mu.Lock()
		draw := c.draw
		c.mu.Unlock()

		if draw == nil {
			continue
		}
		if err := draw(panel); err != nil {
			return fmt.Errorf("display: %w", err)
		}
	}

	if err := panel.Flip(); err != nil {
		return err
	}
	d.mu.Lock()
	unstrand := d.unstrand
	d.unstrand = nil
	d.mu.Unlock()
	if unstrand != nil {
		time.AfterFunc(strandLinger, unstrand)
	}
	return nil
}

func damageOf(stack []*Claim) (Rect, bool) {
	var total Rect
	var any bool

	for _, c := range stack {
		c.mu.Lock()
		d, draw, dirty := c.damage, c.draw, c.dirty
		if draw != nil && dirty {
			c.damage, c.dirty = Rect{}, false
		}
		c.mu.Unlock()

		if draw == nil || !dirty {
			continue
		}

		if !any {
			total, any = d, true
			continue
		}
		total = union(total, d)
	}
	return total, any
}

func union(a, b Rect) Rect {
	if a.W <= 0 || a.H <= 0 {
		return a
	}
	if b.W <= 0 || b.H <= 0 {
		return b
	}

	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func (c *Claim) Refresh(damage Rect) {
	c.mu.Lock()
	released := c.released
	if !released {
		c.grow(damage)
	}
	c.mu.Unlock()

	if !released {
		c.driver.wake()
	}
}

func (c *Claim) grow(damage Rect) {
	if c.dirty {
		damage = union(c.damage, damage)
	}
	c.damage, c.dirty = damage, true
}
