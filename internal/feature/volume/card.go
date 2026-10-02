package volume

import (
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

// How long the card stays up with nobody touching it.
//
// Opened out it stays longer: a button moved a level and the card said so, where opening it out is
// somebody asking to see the rest and then reaching for one of them.
const (
	linger = 5 * time.Second
	opened = 8 * time.Second
)

// card is the volume control over whatever is underneath, and the timer that takes it away.
//
// A view on the shell rather than a claim of its own, because it is something to touch: the shell
// is what turns a contact into a tap on a button or a pull on the bar.
//
// Its lock is taken before the shell's and never the other way round. Nothing the shell calls here
// is called while it holds its own.
type card struct {
	mu    sync.Mutex
	held  *shell.Hold
	timer *time.Timer
	open  bool

	// stream is the one in the circle, which is the one the bar moves.
	stream config.Stream
}

// An overlay: the card hangs off one edge and whatever is underneath carries on showing beside it.
func (c *card) Covers() bool { return false }

// selected is the stream in the circle, and whether the card is up at all. What the volume buttons
// ask, so that pressing one moves the level the card is showing rather than a different one.
func (c *card) selected() (config.Stream, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.held.Held() {
		return "", false
	}
	return c.stream, true
}

// show puts the card up for a stream's level, or brings the card round to that stream when it is
// already up.
//
// Selecting what moved is the whole point: the circle says which level is on the bar, so a button
// press that changed the alerts level must not leave the card showing the media one.
func (c *card) show(ch Change) {
	c.mu.Lock()
	c.stream = ch.Stream

	if !c.held.Held() {
		c.held = shell.Get().Hold(c)
		c.mu.Unlock()

		c.wait()
		return
	}
	c.mu.Unlock()

	repaint()
	c.wait()
}

var repaint = func() { shell.Get().Redraw() }

// stir shows a new level on a card that is already up, and keeps it up.
//
// It has to repaint. The card is itself what Top returns once it is held, so every change after the
// first comes through here rather than through show — and a stir that only reset the timer left the
// card displaying the level it opened at while the volume moved underneath it. A press on the
// device hid that, because Adjust repaints the screen itself afterwards and a phone does not.
//
// Nothing happens when the card is not up: that is the levels page being on top, which outranks the
// card and repaints itself.
func (c *card) stir() {
	c.mu.Lock()
	up := c.held.Held()
	c.mu.Unlock()

	c.stirred(up)
}

// stirred is stir once it knows whether the card is up, split out so the decision can be tested
// without a shell and a panel behind it.
func (c *card) stirred(up bool) {
	if !up {
		return
	}

	repaint()
	c.wait()
}

// wait restarts the countdown, which every change and every touch does. Holding a button keeps the
// card up rather than flickering it away between presses.
func (c *card) wait() {
	c.mu.Lock()
	defer c.mu.Unlock()

	stay := linger
	if c.open {
		stay = opened
	}

	if c.timer == nil {
		c.timer = time.AfterFunc(stay, c.hide)
		return
	}
	c.timer.Reset(stay)
}

// hide takes the card away and folds it back, so the next level that moves brings up the circle
// alone rather than the whole list.
func (c *card) hide() {
	c.mu.Lock()
	held := c.held
	c.held, c.open = nil, false

	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()

	held.Release()
}

// toggle shows the streams the bar is not moving, or folds them away again.
func (c *card) toggle() {
	c.mu.Lock()
	c.open = !c.open
	c.mu.Unlock()

	c.wait()
}

// selects moves the bar to another stream and folds the list away, which is what picking one out
// of a list means: the thing picked becomes the thing shown.
func (c *card) selects(stream config.Stream) func() {
	return func() {
		c.mu.Lock()
		c.stream, c.open = stream, false
		c.mu.Unlock()

		c.wait()
	}
}

// Shows implements the test Set makes before putting the card up. The card already shows whichever
// stream is selected, so a change to that one is something to redraw rather than announce over the
// top; a change to another still brings the card round to it.
func (c *card) Shows(s config.Stream) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.held.Held() && c.stream == s
}

func (v *Volume) Card() shell.View { return &v.card }

func (v *Volume) Picked() (stream config.Stream, open bool) {
	c := &v.card
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stream, c.open
}

func (v *Volume) Pick(s config.Stream) { v.card.selects(s)() }

func (v *Volume) Expand() { v.card.toggle() }

func (v *Volume) Linger() { v.card.wait() }

func (v *Volume) Dismiss() { v.card.hide() }
