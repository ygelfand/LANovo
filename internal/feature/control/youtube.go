package control

import (
	"context"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/lib/cast/protocols/youtube"
	"github.com/ygelfand/LANovo/internal/lib/fetch"
)

func casting() []*cobra.Command {
	yt := group("youtube", "What YouTube serves the panel", "")
	yt.AddCommand(says(&cobra.Command{
		Use:   "formats ID",
		Short: "List a video's formats, and the audio and video the player would pick",
		Args:  cobra.ExactArgs(1),
	}, ytFormats))
	yt.AddCommand(says(&cobra.Command{
		Use:   "live ID [SECONDS] [BACK]",
		Short: "Play a live stream's audio and pictures without showing them, BACK seconds behind now, and report what arrived",
		Args:  cobra.RangeArgs(1, 3),
	}, ytLive))
	return []*cobra.Command{yt}
}

func ytLive(args []string) (string, error) {
	hold, back := 10*time.Second, time.Duration(0)
	for i, d := range []*time.Duration{&hold, &back} {
		if len(args) > i+1 {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return "", err
			}
			*d = time.Duration(n) * time.Second
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), hold+30*time.Second)
	defer cancel()
	return youtube.NewResolver(fetch.Client(30*time.Second)).Probe(ctx, args[0], hold, back)
}

func ytFormats(args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return youtube.NewResolver(fetch.Client(30*time.Second)).Formats(ctx, args[0])
}
