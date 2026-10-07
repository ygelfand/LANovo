package visuals

import "testing"

func TestWithoutTheHelperTheViewWaits(t *testing.T) {
	r, v := newRig()
	w := v.View()
	w.Keep(120, 80)
	if r.tapped() != nil {
		t.Fatal("the unavailable view acquired the audio tap")
	}
}
