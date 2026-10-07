package control

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/a2dp"
	shareda2dp "github.com/ygelfand/libcountertop/pkg/bluetooth/a2dp"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/avrcp"
)

// sink asks the speaker side for things nothing on screen has an opinion about.
//
//	bt trace [ms]   every frame but the audio, while it lasts
//
// The audio is left out because it is the only thing here that arrives by the hundred. Everything
// else on a link is a handful of messages and worth a line each.
func sink(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("bt: %q is not one of its commands", verbOf(args))
	}

	switch args[0] {
	case "trace":
		within := 30 * time.Second
		if len(args) > 1 {
			ms, err := strconv.Atoi(args[1])
			if err != nil || ms < 0 {
				return "", fmt.Errorf("bt: %q is not a number of milliseconds", args[1])
			}
			within = time.Duration(ms) * time.Millisecond
		}

		shareda2dp.Trace(within)
		return fmt.Sprintf("tracing frames for %s, then: adb logcat -s lanovod\n", within), nil

	// Opening the channel is a round trip, so the lookup is two steps: once to ask for the channel,
	// again to ask the question. A single command would send the question before there was
	// anywhere to send it.
	case "look":
		shareda2dp.Trace(10 * time.Second)

		if len(args) < 2 {
			a2dp.Get().Looking()
			return "asked for a lookup channel, then: bt look 110c\n", nil
		}

		class, err := strconv.ParseUint(strings.TrimPrefix(args[1], "0x"), 16, 16)
		if err != nil {
			return "", fmt.Errorf("bt: %q is not a service class", args[1])
		}

		a2dp.Get().Look(uint16(class))
		return fmt.Sprintf("looking up %#04x, watch the trace\n", class), nil

	// Three steps, because each is a round trip: a channel, then a session on it, then the image.
	case "art":
		shareda2dp.Trace(10 * time.Second)

		if len(args) < 2 {
			return "", fmt.Errorf("bt: art PSM | art hello | art HANDLE")
		}

		switch {
		case args[1] == "hello":
			a2dp.Get().Hello()
			return "connecting to the image service, then: bt art HANDLE\n", nil

		// A handle is seven digits and a channel is hex. Long enough to tell apart without asking.
		case len(args[1]) == 7 && isDigits(args[1]):
			a2dp.Get().Thumbnail(args[1])
			return fmt.Sprintf("asked for image %s, watch the trace\n", args[1]), nil
		}

		psm, err := strconv.ParseUint(strings.TrimPrefix(args[1], "0x"), 16, 16)
		if err != nil {
			return "", fmt.Errorf("bt: %q is neither a channel nor a seven digit handle", args[1])
		}

		a2dp.Get().Art(uint16(psm))
		return fmt.Sprintf("asked for the image channel on %#04x, then: bt art hello\n", psm), nil

	case "browse":
		shareda2dp.Trace(10 * time.Second)
		a2dp.Get().Browsing()
		return "asked for the browsing channel, then: adb logcat -s lanovod\n", nil

	case "queue":
		shareda2dp.Trace(10 * time.Second)
		a2dp.Get().Queue()
		return "asked the addressed player what is lined up, then: adb logcat -s lanovod\n", nil

	case "dump":
		within := 5 * time.Second
		if len(args) > 1 {
			secs, err := strconv.Atoi(args[1])
			if err != nil || secs <= 0 {
				return "", fmt.Errorf("bt: %q is not a number of seconds", args[1])
			}
			within = time.Duration(secs) * time.Second
		}
		return a2dp.Get().Dump(within)

	case "attrs":
		ids := []uint32{avrcp.AttrCoverArt}
		if len(args) > 1 {
			ids = nil
			for _, a := range args[1:] {
				n, err := strconv.ParseUint(a, 0, 32)
				if err != nil {
					return "", fmt.Errorf("bt: %q is not an attribute id", a)
				}
				ids = append(ids, uint32(n))
			}
		}

		// The answer comes back on the channel rather than from here, so the trace has to be on to
		// see it. Long enough for the round trip and no longer.
		shareda2dp.Trace(3 * time.Second)
		a2dp.Get().Attributes(ids...)
		return fmt.Sprintf("asked for %v, watch the trace\n", ids), nil
	}
	return "", fmt.Errorf("bt: %q is not one of its commands", verbOf(args))
}

// isDigits reports whether every character is one, which is what tells a seven digit image handle
// from a channel number written in hex.
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
