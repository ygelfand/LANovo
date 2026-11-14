package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

func main() {
	board.Set(board.X501F)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	m := mic.Get()
	if err := m.Start(ctx); err != nil {
		fmt.Println("microphone start:", err)
		return
	}
	fmt.Println("microphone opened")
	if err := m.Run(ctx); err != nil {
		fmt.Println("microphone run:", err)
	}
	_ = m.Close()
}
