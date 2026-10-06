package control

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
)

func calling() []*cobra.Command {
	c := group("call", "Place, answer and end calls to other devices", "")
	c.AddCommand(
		says(&cobra.Command{
			Use:   "dial NAME|ID [video]",
			Short: "Call a device found on the network",
			Args:  cobra.RangeArgs(1, 2),
		}, dial),
		does(&cobra.Command{
			Use:   "answer",
			Short: "Answer the ringing call",
			Args:  cobra.NoArgs,
		}, func([]string) error { call.Get().Answer(); return nil }),
		does(&cobra.Command{
			Use:   "hangup",
			Short: "End or decline the call",
			Args:  cobra.NoArgs,
		}, func([]string) error { call.Get().Hangup(); return nil }),
		does(&cobra.Command{
			Use:       "mute on|off",
			Short:     "Stop or resume sending the microphone",
			Args:      cobra.ExactArgs(1),
			ValidArgs: []string{"on", "off"},
		}, func(args []string) error { call.Get().Mute(args[0] == "on"); return nil }),
		does(&cobra.Command{
			Use:       "camera on|off",
			Short:     "Start or stop sending the camera in a video call",
			Args:      cobra.ExactArgs(1),
			ValidArgs: []string{"on", "off"},
		}, func(args []string) error { call.Get().Camera(args[0] == "on"); return nil }),
		says(&cobra.Command{
			Use:   "state",
			Short: "The call in progress, if any",
			Args:  cobra.NoArgs,
		}, callState),
	)
	return []*cobra.Command{c}
}

func dial(args []string) (string, error) {
	want := strings.ToLower(args[0])
	for _, p := range discovery.Get().Peers() {
		if p.ID == want || strings.ToLower(p.Name) == want {
			if err := call.Get().Start(p, len(args) > 1 && args[1] == "video"); err != nil {
				return "", err
			}
			return "calling " + p.Name, nil
		}
	}
	return "", fmt.Errorf("no device %q on the network", args[0])
}

func callState([]string) (string, error) {
	c, ok := call.Get().Now()
	if !ok {
		return "idle", nil
	}
	out := fmt.Sprintf("%s\t%s\t%s\tvideo=%v\tmuted=%v\tcamera_off=%v\t%s", c.State, c.Peer.Name, c.ID, c.Video, c.Muted, c.CameraOff, time.Since(c.Since).Round(time.Second))
	st, ok := call.Get().Stats()
	if !ok {
		return out, nil
	}
	l := st.Link
	out += fmt.Sprintf("\naudio\tsent=%d recv=%d lost=%d encode=%s", l.AudioSent, l.AudioRecv, l.AudioLost, l.EncodeAverage())
	out += fmt.Sprintf("\nvideo\tsent=%d recv=%d keys=%d waiting=%d key_requests=%d key_asked=%d", l.VideoSent, l.VideoRecv, l.VideoKeys, l.VideoWaiting, l.KeysRequested, l.KeysAsked)
	for _, side := range []struct {
		name string
		s    call.LayerStats
	}{{"remote", st.Remote}, {"self", st.Self}} {
		out += fmt.Sprintf("\n%s\topen=%v %dx%d frames=%d failed=%v", side.name, side.s.Open, side.s.Width, side.s.Height, side.s.Frames, side.s.Failed)
	}
	return out, nil
}
