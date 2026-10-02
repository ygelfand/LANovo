package visuals

import "testing"

func TestWithoutTheHelperTheViewWaits(t *testing.T) {
	r, v := newRig()
	w := v.View()
	w.Keep(120, 80)
	w.mu.Lock()
	running, broken := w.running, w.broken
	w.mu.Unlock()
	if running || broken || r.tapped() != nil {
		t.Fatalf("running %v, broken %v, tapped %v", running, broken, r.tapped() != nil)
	}
}
