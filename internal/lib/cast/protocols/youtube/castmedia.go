package youtube

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cast"
)

const castCommands = 311299

var castThumbs = []cast.Image{
	{URL: "maxresdefault.jpg", Width: 1920, Height: 1080},
	{URL: "hq720.jpg", Width: 1280, Height: 720},
	{URL: "sddefault.jpg", Width: 640, Height: 480},
	{URL: "hqdefault.jpg", Width: 480, Height: 360},
	{URL: "mqdefault.jpg", Width: 320, Height: 180},
	{URL: "default.jpg", Width: 120, Height: 90},
}

type castControl struct{ s *station }

func (c castControl) Load(cast.Media, time.Duration, bool) error {
	return errors.New("youtube: load through the lounge")
}

func (c castControl) Play() error  { go c.s.play(); return nil }
func (c castControl) Pause() error { go c.s.pause(); return nil }
func (c castControl) Stop() error  { go c.s.stop(); return nil }

func (c castControl) Seek(to time.Duration) error { go c.s.seek(to); return nil }
func (c castControl) Step(by int) error           { go c.s.step(by); return nil }

func (c castControl) Elapsed() time.Duration {
	t := c.s.current()
	if t == nil {
		return 0
	}
	return c.s.heard(t)
}

func (s *station) publish(id string, t *track, state int) {
	if s.env.Publish == nil {
		return
	}
	control := castControl{s}
	if state == stateStopped {
		s.env.Publish(control, nil)
		return
	}
	s.mu.Lock()
	list, index := s.list, s.index
	s.mu.Unlock()

	m := cast.Media{ContentID: id, ContentType: "x-youtube/video", StreamType: cast.StreamBuffered}
	m.CustomData, _ = json.Marshal(map[string]any{"listId": list, "currentIndex": index})
	if t != nil {
		m.Metadata = cast.Metadata{Title: t.info.Title, Subtitle: t.info.Author}
		for _, i := range castThumbs {
			i.URL = "https://i.ytimg.com/vi/" + id + "/" + i.URL
			m.Metadata.Images = append(m.Metadata.Images, i)
		}
		if _, _, live := t.window(); live {
			m.StreamType = cast.StreamLive
		} else {
			m.Duration = t.info.Duration.Seconds()
		}
	}

	commands := castCommands
	prev, next := s.around()
	if next {
		commands |= cast.CommandQueueNext
	}
	if prev {
		commands |= cast.CommandQueuePrev
	}
	st := map[int]string{statePlaying: cast.StatePlaying, statePaused: cast.StatePaused, stateLoading: cast.StateBuffering}[state]
	custom, _ := json.Marshal(map[string]int{"playerState": state})
	s.env.Publish(control, &cast.Published{Media: m, State: st, Commands: commands, Custom: custom})
}
