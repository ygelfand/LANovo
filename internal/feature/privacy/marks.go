package privacy

import (
	"log/slog"

	"github.com/ygelfand/LANovo/internal/config"
)

type Marks struct {
	MicMuted      bool
	CameraBlocked bool
}

func (m Marks) Showing() bool { return m.MicMuted || m.CameraBlocked }

func (p *Privacy) Current() Marks {
	if !config.Get().Screen.Marks {
		return Marks{}
	}
	return Marks{MicMuted: p.MicMuted(), CameraBlocked: p.CameraCovered()}
}

func (p *Privacy) show() { p.Changed.Emit(p.Current()) }

func (p *Privacy) SetMarks(on bool) {
	if err := config.Set().Screen().Marks(on); err != nil {
		slog.Error("saving the privacy marks setting failed", "err", err)
		return
	}
	p.show()
}
