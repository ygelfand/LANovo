package privacy

import (
	"log/slog"

	"github.com/ygelfand/LANovo/internal/config"
)

// Marks is what the two physical controls are doing.
type Marks struct {
	MicMuted      bool
	CameraBlocked bool
}

// Showing reports whether there is anything to draw.
func (m Marks) Showing() bool { return m.MicMuted || m.CameraBlocked }

func (p *Privacy) Current() Marks {
	if !config.Get().Screen.Marks {
		return Marks{}
	}
	return Marks{MicMuted: p.MicMuted(), CameraBlocked: p.CameraCovered()}
}

func (p *Privacy) show() { p.Changed.Emit(p.Current()) }

// SetMarks turns the corner marks on or off, and applies it at once.
func (p *Privacy) SetMarks(on bool) {
	if err := config.Set().Screen().Marks(on); err != nil {
		slog.Error("saving the privacy marks setting failed", "err", err)
		return
	}
	p.show()
}
