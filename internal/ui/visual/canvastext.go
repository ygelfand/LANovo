package visual

import "sync"

var canvasText sync.Mutex

func withCanvasText(do func()) {
	canvasText.Lock()
	defer canvasText.Unlock()
	do()
}
