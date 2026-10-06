// Command sbcdecode turns a captured SBC stream into raw PCM, so a decode can be compared against
// a reference implementation rather than judged by ear.
//
// Dev only, like everything under internal/tools: nothing here is imported by the device build.
//
//	go run ./internal/tools/sbcdecode in.sbc out.pcm
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
			// Resynchronise the way the sink does, rather than giving up on the rest.
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

		// Interleaved, which is what every tool that reads a pcm file expects.
		for i := range audio[0] {
			for _, ch := range audio {
				binary.Write(out, binary.LittleEndian, ch[i])
			}
		}

		frames++
		in = in[f.Header.Length():]
	}

	fmt.Printf("%d frames, %d skipped\n", frames, bad)
}
