package visuals

import "testing"

func TestTheLoudestRawLevelIsKept(t *testing.T) {
	var l loudest
	l.hear([]int16{100, -9830, 300})
	l.hear([]int16{50})
	if l.most < 0.299 || l.most > 0.301 {
		t.Errorf("loudest %.3f, want 0.3", l.most)
	}
	l.reset()
	if l.most != 0 {
		t.Errorf("reset left %.3f", l.most)
	}
}
