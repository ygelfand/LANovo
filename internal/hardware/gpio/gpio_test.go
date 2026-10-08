package gpio

import (
	"os"
	"path/filepath"
	"testing"
)

func fake(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, f := range []string{"export", "unexport"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	was := Base
	Base = dir
	t.Cleanup(func() { Base = was })
	return dir
}

func appear(t *testing.T, dir string, n int) {
	t.Helper()

	at := filepath.Join(dir, "gpio"+itoa(n))
	if err := os.MkdirAll(at, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"direction", "value"} {
		if err := os.WriteFile(filepath.Join(at, f), []byte("0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestExportWritesTheNumber(t *testing.T) {
	dir := fake(t)

	if err := (Pin{N: 68}).Export(); err != nil {
		t.Fatalf("Export: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "export"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "68" {
		t.Errorf("export got %q, want 68", b)
	}
}

func TestExportingTwiceIsFine(t *testing.T) {
	dir := fake(t)
	appear(t, dir, 68)

	if err := (Pin{N: 68}).Export(); err != nil {
		t.Errorf("exporting an already exported line failed: %v", err)
	}
}

func TestSetAndGet(t *testing.T) {
	dir := fake(t)
	appear(t, dir, 68)

	p := Pin{N: 68}

	if err := p.Set(true); err != nil {
		t.Fatalf("Set: %v", err)
	}
	on, err := p.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !on {
		t.Error("the line reads low after being driven high")
	}

	if err := p.Set(false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if on, _ := p.Get(); on {
		t.Error("the line reads high after being driven low")
	}
}

func TestSetDirection(t *testing.T) {
	dir := fake(t)
	appear(t, dir, 68)

	p := Pin{N: 68}
	if err := p.SetDirection(Out); err != nil {
		t.Fatalf("SetDirection: %v", err)
	}

	got, err := p.Direction()
	if err != nil {
		t.Fatalf("Direction: %v", err)
	}
	if got != Out {
		t.Errorf("direction = %q, want %q", got, Out)
	}
}

func TestOutput(t *testing.T) {
	dir := fake(t)

	appear(t, dir, 68)

	p, err := Output(68)
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if got, _ := p.Direction(); got != Out {
		t.Errorf("direction = %q, want out", got)
	}
}

func TestDrivingALineThatIsNotThere(t *testing.T) {
	fake(t)

	if err := (Pin{N: 99}).Set(true); err == nil {
		t.Error("driving an unexported line reported success")
	}
	if _, err := (Pin{N: 99}).Get(); err == nil {
		t.Error("reading an unexported line reported success")
	}
}
