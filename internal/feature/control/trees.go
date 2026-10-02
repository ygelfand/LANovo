package control

import (
	"github.com/spf13/cobra"
)

// The commands that have commands of their own.
//
// These were a switch inside a switch, which meant the inner verbs were invisible from outside and
// each one listed its siblings in its own error message. As a tree they list themselves.

// radios is bluetooth and ble, which are separate radios and separate stacks.
func radios() []*cobra.Command {
	bt := group("bt", "The bluetooth audio sink, and what is connected to it",
		"Several of these are two steps, because opening a channel to the far end is a round\n"+
			"trip: the first asks for the channel and the second asks the question. Each one says\n"+
			"what to run next.")
	bt.AddCommand(
		says(&cobra.Command{
			Use:   "trace [MILLISECONDS]",
			Short: "Log frames as they arrive",
			Args:  cobra.MaximumNArgs(1),
		}, under(sink, "trace")),
		says(&cobra.Command{
			Use:   "attrs [ID ...]",
			Short: "Read attributes off the connected player",
			Args:  cobra.ArbitraryArgs,
		}, under(sink, "attrs")),
		says(&cobra.Command{
			Use:   "dump [SECONDS]",
			Short: "Capture the stream to a file",
			Args:  cobra.MaximumNArgs(1),
		}, under(sink, "dump")),
		says(&cobra.Command{
			Use:   "look [CLASS]",
			Short: "Ask the far end what it offers",
			Long:  "Two steps. Without a class it asks for the channel; with one it asks the question.",
			Args:  cobra.MaximumNArgs(1),
		}, under(sink, "look")),
		says(&cobra.Command{
			Use:   "art PSM|hello|HANDLE",
			Short: "Fetch cover art",
			Args:  cobra.ExactArgs(1),
		}, under(sink, "art")),
		says(&cobra.Command{
			Use:   "browse",
			Short: "Walk the player's library",
			Args:  cobra.NoArgs,
		}, under(sink, "browse")),
		says(&cobra.Command{
			Use:   "queue [PLAYER [COUNT]]",
			Short: "Read what is queued up",
			Args:  cobra.MaximumNArgs(2),
		}, under(sink, "queue")),
	)

	ble := group("ble", "The low energy radio", "")
	ble.AddCommand(says(&cobra.Command{
		Use:   "sniff [MILLISECONDS]",
		Short: "Listen for advertisements",
		Args:  cobra.MaximumNArgs(1),
	}, under(radio, "sniff")))

	return []*cobra.Command{bt, ble}
}

// watching is the camera and the player, which are both about what the device can see or is doing.
func watching() []*cobra.Command {
	camera := group("camera", "The imaging hardware", "The vendor camera stack, through lanovo-camera.")
	camera.AddCommand(
		says(&cobra.Command{
			Use:   "still [PATH]",
			Short: "Take the picture Home Assistant would get",
			Long: "Through the same component that answers the camera entity, so this is the whole\n" +
				"path and not a rehearsal of it. Writes to /data/local/tmp/still.jpg unless told\n" +
				"somewhere else.",
			Args: cobra.MaximumNArgs(1),
		}, still),
	)

	return []*cobra.Command{
		camera,
		says(&cobra.Command{
			Use:   "player",
			Short: "What is playing, and on what",
			Args:  cobra.NoArgs,
		}, player),
		says(&cobra.Command{
			Use:   "stack",
			Short: "The screens on the shell's stack, top first, and which are held up",
			Args:  cobra.NoArgs,
		}, stack),
	}
}

// under adapts a handler that still dispatches on its own first argument.
//
// The inner switches are left where they are: they carry the parsing and the two-step dances, and
// moving that here would be a rewrite rather than a reorganisation. This puts the verb back on the
// front so the handler sees what it always saw, while the tree above is what the outside sees.
func under(do func([]string) (string, error), verb string) func([]string) (string, error) {
	return func(args []string) (string, error) {
		return do(append([]string{verb}, args...))
	}
}
