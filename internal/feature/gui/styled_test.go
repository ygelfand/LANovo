package gui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

var styleable = map[string]bool{
	"Switch": true, "Slider": true, "ProgressBar": true, "Button": true, "Input": true,
	"TabControl": true, "Select": true, "Checkbox": true, "Radio": true, "Toggle": true,
	"ListBox": true, "Combobox": true, "NumericInput": true, "DatePicker": true,
}

var unstyled = map[string]string{
	"styled.go:Switch":      "the kit wrappers",
	"styled.go:Slider":      "the kit wrappers",
	"styled.go:ProgressBar": "the kit wrappers",
	"styled.go:ColorPanel":  "the kit wrappers' defaults",
	"look.go:ColorPanel":    "builds the theme the styles draw from",
	"keyboard.go:Input":     "textField applies the style's Field first",
	"rows.go:ColorPanel":    "the camera preview is a hole the video shows through",
}

func TestEveryControlGoesThroughTheStyle(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fs, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			what := sel.Sel.Name
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "gogui" && styleable[what] {
				if _, ok := unstyled[name+":"+what]; !ok {
					t.Errorf("%s: gogui.%s built directly; add the element to style.Kit, implement it in every style, and build it in styled.go", fs.Position(sel.Pos()), what)
				}
			}
			if what == "ColorPanel" {
				if _, ok := unstyled[name+":ColorPanel"]; !ok {
					t.Errorf("%s: a hand-picked panel colour; use panel() or another style.Kit element", fs.Position(sel.Pos()))
				}
			}
			return true
		})
	}
}
