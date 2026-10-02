package control

import (
	"net"
	"strings"
	"testing"
	"time"
)

// What a failing command still gets to say.
//
// A probe works step by step and reports each one, and the step that failed is the least of it:
// what worked before is how the failure is placed. Throwing the output away and printing only the
// error is how a diagnostic that was written turns into a diagnostic nobody ever reads.

// spoken runs one command through serve and returns everything the client saw.
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

	// Read until it stops rather than once: the pipe is unbuffered, so the output and the line that
	// ends it arrive as separate writes.
	var said strings.Builder
	buf := make([]byte, 4096)

	for {
		if err := ours.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}

		n, err := ours.Read(buf)
		said.Write(buf[:n])

		if err != nil {
			break
		}
		if done := strings.TrimSpace(said.String()); strings.HasSuffix(done, "ok") ||
			strings.Contains(done, "error: ") {
			break
		}
	}

	if said.Len() == 0 {
		t.Fatal("nothing came back")
	}
	return said.String()
}

func TestAFailingCommandStillSaysWhatItGotThrough(t *testing.T) {
	// mixer with no name fails outright and has nothing to report, which is the plain case.
	said := spoken(t, "mixer")
	if !strings.Contains(said, "error: ") {
		t.Errorf("a command that failed did not say so: %q", said)
	}
}

func TestOutputComesBeforeTheError(t *testing.T) {
	// A command that does not exist at all: an error and nothing else, and no "ok" after it.
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

	// Every command, named once, from the commands themselves rather than a list beside them.
	for _, want := range []string{"tap", "camera", "bt", "shot", "mixer", "player"} {
		if !strings.Contains(said, want) {
			t.Errorf("help did not mention %q: %q", want, said)
		}
	}

	// Cobra offers a shell completion generator, which is noise for something driven over a socket.
	if strings.Contains(said, "completion") {
		t.Errorf("help offered shell completion: %q", said)
	}
	if !strings.HasSuffix(strings.TrimSpace(said), "ok") {
		t.Errorf("a command that worked did not end with ok: %q", said)
	}
}

// A command that only holds others answers a name it does not have with an error, rather than
// printing its help and calling that success. A typo should not look like it worked.
func TestAMistypedSubcommandFails(t *testing.T) {
	for _, line := range []string{"bt nonesuch", "camera nonesuch", "ble nonesuch"} {
		said := spoken(t, line)

		if !strings.Contains(said, "error: ") {
			t.Errorf("%q did not fail: %q", line, said)
		}
		if strings.HasSuffix(strings.TrimSpace(said), "ok") {
			t.Errorf("%q was reported as ok: %q", line, said)
		}
	}
}

// Naming one of the trees on its own is help, not an error.
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
