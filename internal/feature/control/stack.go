package control

import (
	"fmt"
	"strings"

	"github.com/ygelfand/LANovo/internal/feature/shell"
)

func stack([]string) (string, error) {
	views := shell.Get().Stack()
	if len(views) == 0 {
		return "empty", nil
	}
	var b strings.Builder
	for i := len(views) - 1; i >= 0; i-- {
		v := views[i]
		where := ""
		if i == len(views)-1 {
			where = " (top)"
		}
		title := ""
		if p, ok := v.View.(*shell.Page); ok {
			title = fmt.Sprintf(" %q", p.Title)
		}
		fmt.Fprintf(&b, "%d %T%s held=%v covers=%v%s\n", i, v.View, title, v.Held, v.View.Covers(), where)
	}
	return b.String(), nil
}
