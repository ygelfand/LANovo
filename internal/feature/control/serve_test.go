package control

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
)

func spoken(t *testing.T, line string) string {
	t.Helper()

	ours, theirs := net.Pipe()
	t.Cleanup(func() { ours.Close(); theirs.Close() })

	go (&Control{}).serve(theirs)

	if err := ours.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := ours.Write([]byte(line + "\n")); err != nil {
		t.Fatal(err)
	}

	var said strings.Builder
	if err := harness.Answer(bufio.NewScanner(ours), &said); err != nil {
		return said.String() + "error: " + err.Error()
	}
	return said.String() + "ok"
}

func TestAFailingCommandStillSaysWhatItGotThrough(t *testing.T) {
	said := spoken(t, "audio mixer")
	if !strings.Contains(said, "error: ") {
		t.Errorf("a command that failed did not say so: %q", said)
	}
}

func TestOutputComesBeforeTheError(t *testing.T) {
	said := spoken(t, "nonesuch")

	if !strings.Contains(said, "error: ") {
		t.Errorf("an unknown command did not fail: %q", said)
	}
	if strings.Contains(said, "ok") {
		t.Errorf("a command that failed was reported as ok: %q", said)
	}
}

func TestASucceedingCommandEndsWithOk(t *testing.T) {
	said := spoken(t, "help")

	for _, want := range []string{"tap", "camera", "bluetooth", "shot", "mixer", "player"} {
		if !strings.Contains(said, want) {
			t.Errorf("help did not mention %q: %q", want, said)
		}
	}

	if strings.Contains(said, "completion") {
		t.Errorf("help offered shell completion: %q", said)
	}
	if !strings.HasSuffix(strings.TrimSpace(said), "ok") {
		t.Errorf("a command that worked did not end with ok: %q", said)
	}
}

func TestAMistypedSubcommandFails(t *testing.T) {
	for _, line := range []string{"bluetooth nonesuch", "camera nonesuch", "ble nonesuch"} {
		said := spoken(t, line)

		if !strings.Contains(said, "error: ") {
			t.Errorf("%q did not fail: %q", line, said)
		}
		if strings.HasSuffix(strings.TrimSpace(said), "ok") {
			t.Errorf("%q was reported as ok: %q", line, said)
		}
	}
}

func TestATreeOnItsOwnIsHelp(t *testing.T) {
	said := spoken(t, "camera")

	if strings.Contains(said, "error: ") {
		t.Errorf("naming a tree failed: %q", said)
	}
	for _, want := range []string{"still", "Available Commands"} {
		if !strings.Contains(said, want) {
			t.Errorf("help for the tree did not mention %q: %q", want, said)
		}
	}
}
