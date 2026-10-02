package face

import "github.com/ygelfand/LANovo/internal/ui"

// fit is the largest size at which the text is no wider than maxW, starting from maxH.
//
// Shared, because every face that sets type has the same problem: the box is the constraint and the
// size is the answer, and a face that guessed a size would either overrun the panel or leave it
// half empty.
//
// Width grows with size, so scaling the size by how much it overran lands close on the first try;
// the loop is there because rounding and hinting mean close is not always under.
func fit(weight ui.Weight, text string, maxW, maxH int) int {
	size := max(maxH, 1)

	for range 8 {
		w, _ := ui.MustLoad(weight, size).Measure(text)
		if w <= maxW || size <= 1 {
			return size
		}

		next := size * maxW / w
		if next >= size {
			next = size - 1
		}
		size = max(next, 1)
	}
	return size
}
