package takeover

import (
	"path"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/layout"
)

var Stashed = []string{"/system/app", "/system/priv-app"}

type move struct{ from, to string }

func moves(listing string) []move {
	var out []move
	for _, line := range strings.Split(listing, "\n") {
		p := strings.TrimSpace(line)
		dir, name := path.Split(p)
		dir = strings.TrimSuffix(dir, "/")
		for _, s := range Stashed {
			if dir == s && name != "" && name != "*" {
				out = append(out, move{from: p, to: path.Join(layout.Stash, path.Base(s), name)})
			}
		}
	}
	return out
}

func Stash(d *adb.Device) ([]Step, error) {
	listing, _ := d.Shell("ls -d " + strings.Join(Stashed, "/* ") + "/* 2>/dev/null")
	todo := moves(listing)
	if len(todo) == 0 {
		return nil, nil
	}

	restore, err := Writable(d)
	if err != nil {
		return nil, err
	}
	defer restore()

	var steps []Step
	for _, m := range todo {
		if _, err := d.Shell(
			"mkdir -p " + path.Dir(m.to) + " && rm -rf " + m.to + " && mv " + m.from + " " + m.to,
		); err != nil {
			return steps, err
		}
		steps = append(steps, Step{What: "moved", Note: m.from + " → " + m.to})
	}
	return steps, nil
}
