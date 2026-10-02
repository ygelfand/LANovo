package glow

import (
	"runtime"
	"sync"
)

const rowsAlone = 1 << 16

func Rows(y0, y1, w int, f func(y0, y1 int)) {
	n := min(runtime.GOMAXPROCS(0), y1-y0)
	if n < 2 || (y1-y0)*w < rowsAlone {
		f(y0, y1)
		return
	}
	var wg sync.WaitGroup
	for i := range n {
		a, b := y0+(y1-y0)*i/n, y0+(y1-y0)*(i+1)/n
		wg.Go(func() { f(a, b) })
	}
	wg.Wait()
}
