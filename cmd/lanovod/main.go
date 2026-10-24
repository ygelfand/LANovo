package main

import (
	"github.com/ygelfand/libcountertop/pkg/runtime/payload"

	"github.com/ygelfand/LANovo/internal/cli/lanovod"
)

func main() {
	payload.Enter()
	lanovod.Execute()
}
