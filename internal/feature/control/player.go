package control

import (
	"fmt"
	"strings"

	"github.com/ygelfand/LANovo/internal/feature/media"
)

// What the player thinks is playing.
//
// A blank card has several causes that look the same from outside: nobody holding it, a holder that
// is not the one being heard, a holder with nothing to say.
func player(args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("player takes no arguments")
	}

	now := media.Get().Now()
	source, external := media.Get().Sourced()

	var b strings.Builder
	fmt.Fprintf(&b, "source      %s\n", source)
	fmt.Fprintf(&b, "external    %v\n", external)
	fmt.Fprintf(&b, "playing     %v\n", now.Playing)
	fmt.Fprintf(&b, "paused      %v\n", now.Paused)
	fmt.Fprintf(&b, "title       %q\n", now.Title)
	fmt.Fprintf(&b, "artist      %q\n", now.Artist)
	fmt.Fprintf(&b, "album       %q\n", now.Album)
	fmt.Fprintf(&b, "art         %v\n", now.Art != nil)
	fmt.Fprintf(&b, "controls    %04b\n", now.Can)

	return b.String(), nil
}
