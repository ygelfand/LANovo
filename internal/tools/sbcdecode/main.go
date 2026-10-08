package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/ygelfand/libcountertop/pkg/bluetooth/sbc"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: sbcdecode in.sbc out.pcm")
		os.Exit(2)
	}

	in, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	out, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer out.Close()

	var filter sbc.Filter
	frames, bad := 0, 0

	for len(in) > 0 {
		f, err := sbc.Unpack(in)
		if err != nil {
			at, ok := sbc.Find(in[1:])
			if !ok {
				break
			}
			in = in[1+at:]
			bad++
			continue
		}

		audio, err := filter.Synthesize(f)
		if err != nil {
			bad++
			break
		}

		for i := range audio[0] {
			for _, ch := range audio {
				if err := binary.Write(out, binary.LittleEndian, ch[i]); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
			}
		}

		frames++
		in = in[f.Header.Length():]
	}

	fmt.Printf("%d frames, %d skipped\n", frames, bad)
}
