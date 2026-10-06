package control

import (
	"fmt"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/discovery"
)

func peers([]string) (string, error) {
	var out strings.Builder
	for _, p := range discovery.Get().Peers() {
		addrs := make([]string, 0, len(p.Addrs))
		for _, a := range p.Addrs {
			addrs = append(addrs, a.String())
		}
		fmt.Fprintf(&out, "%s\t%s\t%s\t%s\t%s:%d\t%s\tv%s\t%s ago\n",
			p.ID, p.Name, p.Model, p.Board, strings.Join(addrs, ","), p.Port,
			strings.Join(p.Caps, ","), p.Version, time.Since(p.Seen).Round(time.Second))
	}
	if out.Len() == 0 {
		return "none", nil
	}
	return out.String(), nil
}
