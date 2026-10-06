// Package display owns the panel.
//
// One framebuffer, held for the life of the process. Everything that wants to draw takes a claim
// rather than writing to it, because the panel is one surface with several legitimate claimants and
// without an order they overwrite each other in whatever sequence events happened to arrive.
package display

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	// First of the hardware: the boot screen is the only thing the device can say before anything
	// else works.
	component.Register(component.Hardware, Get, component.Order(5),
		component.Supervise(service.Restart(time.Second, 30*time.Second)))
}

// Priority is how the panel resolves being asked for two things at once. Higher wins, and when a
// claim goes away whatever is under it comes back on its own.
type Priority int

const (
	// PriorityDashboard is what the screen shows when nothing is happening.
	PriorityDashboard Priority = iota

	// PrioritySetup is a device that cannot do its job yet and is asking to be set up. It sits
	// over the dashboard because a clock is not what someone standing in front of an unconfigured
	// device needs to see.
	PrioritySetup

	// PriorityUI is a screen someone opened and is touching. It covers the dashboard, and a
	// notice still shows over it: turning the volume while the settings are open should say so.
	PriorityUI

	// PriorityNotice is a brief acknowledgement, such as a volume change.
	PriorityNotice

	// PriorityAlert is something the user has to see. It outranks a notice so that dismissing the
	// notice cannot take the alert with it.
	PriorityAlert

	// PriorityBoot is start-up. Nothing else may write while it holds.
	PriorityBoot
)

// Driver owns the panel. One goroutine writes to the hardware; everything else takes a Claim and
// says what it wants, which removes the question of who painted last.
type Driver struct {
	path string

	// damaged is the part of the screen the last frame repainted, kept because the buffer drawn
	// into now is the one from the frame before that.
	damaged Rect

	// forced is a repaint nothing reported: the panel turning, or the stack changing.
	forced bool

	dropped error

	opened   bool
	stranded func(*Panel) error
	unstrand func()

	// parting is set once a last frame is up: the picture stays until the process is gone, and
	// anything still drawing on its way out does not get to land over it.
	parting bool

	// What rendering has cost, for working out where a frame goes.
	frames, whole int
	painted       int64

	Released hook.Hook[Priority]

	mu     sync.Mutex
	panel  *Panel
	claims []*Claim

	// info is what the panel says about itself, kept here because the boot screen asks for it
	// while the render goroutine is drawing, and the panel's own geometry is what drawing reads.
	info string

	brightness int

	// rot is what a panel is opened at and set to, held here because the panel may not be open
	// when the device is turned.
	rot Orientation

	// changed wakes the render loop. Buffered, so a change that lands between choosing what to
	// render and starting to watch for changes is not lost.
	changed chan struct{}

	// shots are screenshot requests, served by the render loop between frames so a capture cannot
	// land in the middle of one. Unbuffered: the send is what waits for the loop to be free.
	shots chan chan capture

	// lasts are final frames. Unbuffered, like shots: the send waits for the loop.
	lasts chan lastFrame

	// benches are frame rate measurements, run on the render goroutine because that is the thing
	// being measured. Unbuffered, like the rest.
	benches chan benchRequest
}

// benchRequest asks the render loop to time itself, and says where to put the answer.
type benchRequest struct {
	frames int

	// stop is when to give up whatever is left. The loop draws nothing else while this runs, so
	// the deadline has to be honoured by the thing holding it and not only by the caller waiting.
	stop time.Time

	done chan benchResult
}

type benchResult struct {
	bench Bench
	err   error
}

// Bench is what a run of frames cost, split where the cost actually divides: drawing into the back
// buffer, and putting that buffer on the panel.
type Bench struct {
	Frames    int
	Draw, Pan time.Duration
	Width     int
	Height    int
}

// Each is the average cost of one frame.
func (b Bench) Each() (draw, pan time.Duration) {
	if b.Frames == 0 {
		return 0, 0
	}
	return b.Draw / time.Duration(b.Frames), b.Pan / time.Duration(b.Frames)
}

// Rate is the frames a second this pipeline would sustain if it did nothing else.
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

// lastFrame is a picture painted outside the claim stack, and somewhere to say it landed.
type lastFrame struct {
	draw func(*Panel) error
	done chan error
}

// capture is one screenshot, or why there is not one.
type capture struct {
	pixels []byte
	w, h   int
	err    error
}

var (
	once   sync.Once
	shared *Driver
)

// Get is the panel. There is one, and everything that wants to show something takes a claim on it
// rather than owning it.
//
// Nothing here opens the device: a constructor runs in every invocation of the binary, including
// tools and tests on a machine with no panel. Start takes the hardware.
func Get() *Driver { once.Do(func() { shared = NewDriver(layout.FBDevice) }); return shared }

// NewDriver is a driver over one framebuffer, for a tool that wants its own.
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

// Startup is ready once the panel is open, which is the one thing everything else on screen
// depends on.
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

// Start takes the framebuffer, and again on every restart: it may have been taken while we were
// gone.
func (d *Driver) Start(ctx context.Context) error {
	p, err := d.open(ctx)
	if err != nil {
		return err
	}

	d.mu.Lock()
	brightness, rot := d.brightness, d.rot
	d.mu.Unlock()

	// A panel opens at how the device is mounted; the device may have been turned since. Before it
	// is published, so nothing else can be looking at it while this writes its geometry.
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

// Covered reports whether anything below a priority has something to draw.
//
// Releasing a claim does not clear the panel: what was last drawn stays until something draws
// over it. So whatever holds the screen at start-up has to know there is something underneath
// before it lets go, or the device is left showing a picture nobody is maintaining.
//
// The same question render asks, with a ceiling on it. Asking it twice is what let the two
// disagree: this said an empty claim was not worth handing over to while render was picking one.
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

// Stats is what rendering has cost since the driver started: how many frames, how many of those
// repainted everything, and how many pixels were painted in total.
func (d *Driver) Stats() (frames, whole int, painted int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.frames, d.whole, d.painted
}

// shotWait is how long to give the render loop before giving up on it. A frame is milliseconds;
// this is long enough that a slow one is waited for and short enough that a wedged loop is
// reported rather than hanging whoever asked. A variable so a test does not have to sit through it.
var shotWait = 2 * time.Second

// Shot is what is on the panel, as RGB triples read row by row in viewed coordinates.
//
// Handed to the render loop rather than read here. Reading the front buffer from another goroutine
// races the flip, and the result is two or three frames blended down the screen — which is fine
// for a settled picture and useless for anything caught mid-interaction, which is exactly what a
// screenshot is usually for.
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

// Orientation is how the picture currently sits on the panel.
func (d *Driver) Orientation() Orientation {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rot
}

// SetOrientation turns the picture, and everything drawn after it, to face the other way.
//
// The panel is not turned here. Turning writes the geometry that drawing reads for every pixel, and
// this is called from whatever noticed the device move; render applies it instead, on the one
// goroutine that touches the panel at all.
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

// Brightness sets the panel's backlight, as a percentage, and remembers it across a restart.
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

// Run drives the panel until ctx is canceled. Nothing reaches the hardware without going through
// it.
func (d *Driver) Run(ctx context.Context) error {
	d.mu.Lock()
	open := d.panel != nil
	d.mu.Unlock()

	// Said rather than worked around. Running without the panel is a display that reports itself
	// up and draws nothing, and one that never returns is never restarted either.
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

// capture takes the copy. Only ever called from Run, between frames, which is the whole point:
// read anywhere else and the panel can flip halfway down it.
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

// Last paints one frame and waits for it to reach the panel, then stops rendering: whatever was on
// its way to the screen is not going to arrive over it.
//
// For what the process puts up on its way out. The frame is drawn outside the claim stack, because
// the point is that nothing underneath it is consulted and nothing above it can take over.
//
// It lasts until the process exits, and no longer: the panel is blank by the time the next one
// opens it. This is the last thing seen before the gap, not something that fills it.
//
// A driver with no render loop answers straight away with an error rather than waiting: a frame
// nobody is going to draw is not worth holding up a shutdown for.
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

// paint draws one frame straight onto the panel, and shuts rendering down behind it.
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

	// Only once it is actually up: a frame that did not make it is no reason to stop drawing.
	d.mu.Lock()
	d.parting = true
	d.mu.Unlock()
	return nil
}

// Bench draws what is already on screen over and over and reports what it cost.
//
// The real pipeline, in the running process, because that is the only place it can be measured: the
// panel is owned by one goroutine here, and a second process on /dev/graphics/fb0 measures
// something else. It draws the claims that are up rather than a test pattern, so the answer is
// about the screen the device actually shows.
//
// Nothing visible changes. The same picture goes into both buffers, and the frame counters are left
// alone so a measurement does not land in what #135 is watching.
func (d *Driver) Bench(ctx context.Context, frames int) (Bench, error) {
	if frames <= 0 {
		return Bench{}, fmt.Errorf("display: ask for at least one frame")
	}
	done := make(chan benchResult, 1)

	// The loop's own deadline, so it stops drawing when the caller stops waiting rather than
	// holding the panel for frames nobody will read.
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

// measure runs the frames, timing the two halves apart. On the render goroutine.
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
			// The claims' dirty flags are deliberately not cleared: this is measuring the drawing,
			// not showing anything, and swallowing a repaint somebody asked for would be a bug
			// that only appeared while benchmarking.
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

// Close lets go of the framebuffer, leaving on screen whatever was drawn.
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

// Claim asks for the panel at a priority, for something that covers it. Nothing is shown until the
// claim is given something to draw, and whatever it draws lasts until it is released or something
// higher takes over.
func (d *Driver) Claim(p Priority) *Claim { return d.claim(p, true) }

// Overlay is a claim that draws over what is beneath rather than replacing it: a volume column, a
// drawer, a card. Everything under it is drawn first, so it composites onto what is really there.
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

// Claim is one thing's hold on the panel. It is safe to use from any goroutine, and safe to keep
// after releasing: everything on a released claim does nothing.
type Claim struct {
	driver   *Driver
	priority Priority

	// covers says the claim paints the whole panel. One that does not is drawn on top of whatever
	// is under it, which has to be drawn first.
	covers bool

	mu       sync.Mutex
	draw     func(*Panel) error
	released bool

	// dirty is whether the claim has changed since it was drawn, damage which part of it. They
	// are separate because nothing changed and everything changed are different answers.
	dirty  bool
	damage Rect
}

// Show gives the claim something to draw. draw is called on the render goroutine, so it must not
// block on anything slow.
func (c *Claim) Show(draw func(*Panel) error) {
	c.mu.Lock()
	c.draw = draw

	// All of it, which is what a caller that has not been taught to say otherwise means.
	c.damage, c.dirty = Rect{}, true

	released := c.released
	c.mu.Unlock()

	if !released {
		c.driver.wake()
	}
}

// ShowIn gives the claim something to draw and says which part of the panel it changes, so the
// repaint costs that rectangle rather than the screen.
//
// The rectangle has to hold every pixel that differs from the frame before, including the ones
// below an overlay, because everything is clipped to it. Declaring one too small leaves a stale
// strip that no later repaint corrects. ui.Changed is how that is checked in a test rather than on
// the panel.
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

// Clear gives up the surface without releasing the claim. What it covered is drawn again, the same
// as a release: a claim with nothing to draw names no damage of its own.
func (c *Claim) Clear() {
	c.Show(nil)
	d := c.driver
	d.mu.Lock()
	d.forced = true
	d.mu.Unlock()
	d.wake()
}

// Release gives the panel back to whatever was underneath, which has to be drawn again.
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

// topmost is the claim that gets the panel: the highest priority with something to draw, and the
// most recent of those when several share it.
func (d *Driver) topmost() *Claim { return d.top(anyPriority) }

// Showing is the priority of whatever is on the panel, and whether anything is.
//
// What it is for: telling a device that has finished starting up from one still on the boot
// screen. Nothing else can answer that from outside — the control socket is listening long before
// the dashboard has the panel, so a harness that waits for the socket is waiting for the wrong
// thing and runs its first step against the splash.
func (d *Driver) Showing() (Priority, bool) {
	top := d.topmost()
	if top == nil {
		return 0, false
	}
	return top.priority, true
}

// stack is everything to draw, lowest first: the highest claim that covers the panel, and every
// overlay above it.
//
// clear says nothing covering was found, so the panel is painted out before the overlays go on.
// Without it an overlay would land on whatever happened to be in the buffer, which on a double
// buffered panel is the frame before last.
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

	// By priority, keeping registration order within one: the most recent of equals ends up last
	// and so draws over the others.
	slices.SortStableFunc(showing, func(a, b *Claim) int { return cmp.Compare(a.priority, b.priority) })

	for i := len(showing) - 1; i >= 0; i-- {
		if showing[i].covers {
			return showing[i:], false
		}
	}
	return showing, true
}

// anyPriority is a ceiling no claim reaches, for asking without one.
const anyPriority = PriorityBoot + 1

// top is the claim that would get the panel if nothing at or above ceiling existed.
//
// A claim holding nothing is passed over rather than blocking what is under it. Clear leaves one
// in exactly that state, and it is meant to give the surface back.
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

// Native is the panel's size as the controller fetches it, or zero while there is no panel.
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

// wake asks the render loop to look again. Never blocks: a full channel already means it will.
func (d *Driver) wake() {
	select {
	case d.changed <- struct{}{}:
	default:
	}
}

// render draws the claims that are showing, lowest first.
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

	// Here rather than where the device was noticed turning: this is the only goroutine that
	// touches the panel, and turning writes what drawing reads for every pixel.
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

	// Flipping without drawing would put up the buffer from two frames ago.
	if !any && !forced {
		return nil
	}

	if forced {
		now = Rect{}
	}

	// With the previous frame's: the buffer being drawn into was last drawn two frames ago, so
	// what changed then was painted into the other one.
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

	// Overlays with nothing under them would otherwise sit on whatever the buffer last held.
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

	// Drawing goes to the back buffer, so nothing is on the panel until it is flipped. Here
	// rather than in the drawing: every claim would otherwise have to remember, and one that
	// forgot would draw perfectly into a buffer nobody sees.
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

// damageOf is the part of the screen the stack says changed, and whether anything did. A claim
// cannot know what is under it, so a partial repaint needs every dirty claim to have named one.
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

		// From the first real rectangle: union reads a zero one as the whole screen.
		if !any {
			total, any = d, true
			continue
		}
		total = union(total, d)
	}
	return total, any
}

// union is the smallest rectangle holding both. An empty one means the whole panel, so it wins.
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

// Refresh draws the claim again, saying only this part of it changed. A claim that is not sure
// uses Show.
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
