package config

import "github.com/ygelfand/LANovo/internal/lib/say"

type Idle struct {
	After    Delay      `json:"after"`
	Media    Delay      `json:"media"`
	Face     Face       `json:"face"`
	Position Position   `json:"position"`
	Align    Align      `json:"align"`
	Size     Size       `json:"size"`
	First    IdleVisual `json:"first"`
	Second   IdleVisual `json:"second"`
}

type IdleVisual struct {
	Kind   string `json:"kind"`
	Source Source `json:"source"`
}

func (v IdleVisual) On() bool { return v.Kind != "" }

const FaceNone Face = "none"

func IdleFaces() []Face { return append([]Face{FaceNone}, Faces()...) }

type Align string

const (
	AlignLeft   Align = "left"
	AlignCenter Align = "center"
	AlignRight  Align = "right"
)

func (a Align) Label() string {
	switch a {
	case AlignLeft:
		return say.T("align.left")
	case AlignCenter:
		return say.T("align.center")
	case AlignRight:
		return say.T("align.right")
	}
	return string(a)
}

func Aligns() []Align { return []Align{AlignLeft, AlignCenter, AlignRight} }

type Source string

const (
	SourceBoth    Source = "both"
	SourceMic     Source = "mic"
	SourceSpeaker Source = "speaker"
)

func (s Source) Label() string {
	switch s {
	case SourceBoth:
		return say.T("source.both")
	case SourceMic:
		return say.T("source.mic")
	case SourceSpeaker:
		return say.T("source.speaker")
	}
	return string(s)
}

func Sources() []Source { return []Source{SourceBoth, SourceMic, SourceSpeaker} }

func defaultIdle() Idle {
	return Idle{
		After:    DefaultDelay,
		Media:    Delay5m,
		Face:     DefaultFace,
		Position: DefaultPosition,
		Align:    AlignCenter,
		Size:     DefaultSize,
		First:    IdleVisual{Source: SourceBoth},
		Second:   IdleVisual{Source: SourceBoth},
	}
}

type IdleWriter struct{ st *Store }

func (w IdleWriter) After(v Delay) error {
	return w.st.Update(func(c *Config) { c.Idle.After = v })
}

func (w IdleWriter) Media(v Delay) error {
	return w.st.Update(func(c *Config) { c.Idle.Media = v })
}

func (w IdleWriter) Face(v Face) error {
	return w.st.Update(func(c *Config) { c.Idle.Face = v })
}

func (w IdleWriter) Position(v Position) error {
	return w.st.Update(func(c *Config) { c.Idle.Position = v })
}

func (w IdleWriter) Align(v Align) error {
	return w.st.Update(func(c *Config) { c.Idle.Align = v })
}

func (w IdleWriter) Size(v Size) error {
	return w.st.Update(func(c *Config) { c.Idle.Size = v })
}

func (w IdleWriter) Visual(slot int, v IdleVisual) error {
	return w.st.Update(func(c *Config) {
		if slot == 0 {
			c.Idle.First = v
		} else {
			c.Idle.Second = v
		}
	})
}
