package videoplayer

import (
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/media"
)

type fake struct {
	now    media.Now
	paused int
}

func (f *fake) Play()              { f.now.Paused = false }
func (f *fake) Pause()             { f.now.Paused = true; f.paused++ }
func (f *fake) Stop()              {}
func (f *fake) Next()              {}
func (f *fake) Previous()          {}
func (f *fake) Now() media.Now     { return f.now }
func (f *fake) Kind() media.Kind   { return media.FromCast }
func (f *fake) Label() string      { return "phone" }
func (f *fake) Seek(time.Duration) {}
func (f *fake) CanSeek() bool      { return true }

func playing() *fake {
	return &fake{
		now: media.Now{
			Playing: true,
			Title:   "a video",
			Elapsed: 95 * time.Second,
			Length:  14 * time.Minute,
		},
	}
}

func TestFollowPointsTheControlsAtTheNextTrack(t *testing.T) {
	first, next := playing(), playing()
	p := NewPage(first)
	p.Follow(next)
	p.Look().Controls.Pause()
	if first.paused != 0 || next.paused != 1 {
		t.Errorf("paused the first track %d times and the next %d", first.paused, next.paused)
	}
}

func TestRevealAndConcealToggleTheControls(t *testing.T) {
	p := NewPage(playing())
	if p.Look().Shown {
		t.Fatal("controls showing before anything asked for them")
	}
	p.Reveal()
	if !p.Look().Shown {
		t.Fatal("reveal left the controls hidden")
	}
	p.Conceal()
	if p.Look().Shown {
		t.Error("conceal left the controls showing")
	}
}

func TestAPictureEndsTheLoadingSpinner(t *testing.T) {
	p := NewPage(playing())
	p.SetPicture(true)
	if look := p.Look(); !look.Picture || look.Spinning {
		t.Errorf("picture %v, spinning %v", look.Picture, look.Spinning)
	}
}
