package board

import "testing"

func TestSecureVideo(t *testing.T) {
	tests := []struct {
		board Board
		want  bool
	}{
		{Blueberry, true},
		{Opal, true},
		{Ivy, true},
		{Penghu, false},
		{Board{}, false},
	}
	for _, tt := range tests {
		if got := tt.board.Has(SecureVideo); got != tt.want {
			t.Errorf("%s.Has(SecureVideo) = %v, want %v", tt.board.Name, got, tt.want)
		}
	}
	if !Penghu.Has(0) {
		t.Errorf("Penghu.Has(0) = false, want true")
	}
}
