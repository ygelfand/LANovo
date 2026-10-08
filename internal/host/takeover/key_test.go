package takeover

import (
	"errors"
	"strings"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"
)

func generated(t *testing.T) string {
	t.Helper()

	k, err := esphome.GeneratePSK()
	if err != nil {
		t.Fatal(err)
	}
	return k.String()
}

func TestAKeyIsReadTheWayItIsWritten(t *testing.T) {
	key := generated(t)

	for _, file := range []string{key, key + "\n", "  " + key + "\n\n", key + "\r\n"} {
		got, err := parseKey([]byte(file))
		if err != nil {
			t.Errorf("%q: %v", file, err)
			continue
		}
		if got != key {
			t.Errorf("read %q, want %q", got, key)
		}
	}
}

func TestNothingWrittenYetIsItsOwnAnswer(t *testing.T) {
	for _, file := range []string{"", "\n", "   \t\n"} {
		_, err := parseKey([]byte(file))
		if !errors.Is(err, ErrNoKey) {
			t.Errorf("%q gave %v, want ErrNoKey", file, err)
		}
	}
}

func TestAKeyThatIsNotOneIsRefused(t *testing.T) {
	key := generated(t)

	tests := []struct {
		name string
		file string
	}{
		{"truncated", key[:20]},
		{"one character short", key[:len(key)-2] + "="},
		{"not base64", strings.Repeat("!", len(key))},
		{"too few bytes", "aGVsbG8="},
		{"a log line that ended up in the file", "I lanovod: generated an encryption key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseKey([]byte(tt.file))
			if err == nil {
				t.Fatalf("accepted %q as a key", got)
			}
			if errors.Is(err, ErrNoKey) {
				t.Error("reported as missing rather than as wrong, so it would be waited for")
			}
		})
	}
}

func TestARefusalSaysWhereTheKeyIs(t *testing.T) {
	_, err := parseKey([]byte("not a key"))
	if err == nil {
		t.Fatal("accepted a key that is not one")
	}
	if !strings.Contains(err.Error(), "/psk") {
		t.Errorf("error is %q, and does not say which file", err)
	}
}

func TestAGeneratedKeyReadsBack(t *testing.T) {
	key := generated(t)

	got, err := parseKey([]byte(key + "\n"))
	if err != nil {
		t.Fatalf("a freshly generated key did not read back: %v", err)
	}
	if got != key {
		t.Errorf("read %q, want %q", got, key)
	}
}
