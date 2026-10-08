package main

import (
	"fmt"
	"os"

	"github.com/ygelfand/libcountertop/pkg/build/native"

	"github.com/ygelfand/LANovo/internal/board"
)

func main() {
	name := ""
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: native-target [board]")
		os.Exit(1)
	}
	if len(os.Args) == 2 {
		name = os.Args[1]
	}
	var targets []native.Target
	for _, b := range board.All() {
		targets = append(targets, native.Target{Name: b.Name, API: b.NativeAPI})
	}
	api, err := native.Select(targets, name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(api)
}
