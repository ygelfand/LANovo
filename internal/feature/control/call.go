package control

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"

	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
)

func calling() []*cobra.Command {
	c := call.Get()
	return []*cobra.Command{harness.CallCommand(harness.CallActions{Dial: dial, State: callState, Answer: c.Answer, Hangup: c.Hangup, Mute: c.Mute, Camera: c.Camera})}
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
