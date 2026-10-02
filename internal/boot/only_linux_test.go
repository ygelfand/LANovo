package boot

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// Linux only, and not because of a build convenience: an abstract socket is a Linux thing, and on
// anything else net.Listen takes "@lanovod-instance" as a filename and leaves one lying about.

// The lock has to actually exclude, which is the whole point of it.
func TestTheSecondOneIsRefused(t *testing.T) {
	done, err := Only()
	if err != nil {
		t.Skipf("something already holds %s: %v", instance, err)
	}
	defer done()

	again, err := Only()
	if err == nil {
		again()
		t.Fatal("two processes took the lock at once")
	}

	var taken *Taken
	if !errors.As(err, &taken) {
		t.Fatalf("the error is %T, off which the exit code cannot be picked", err)
	}

	// This process is the holder and the one being told, which is the case naming it has to get
	// right: the pid is there in /proc and the walk has to find it.
	if taken.Pid != os.Getpid() {
		t.Errorf("the holder is pid %d, want this process at %d", taken.Pid, os.Getpid())
	}
	if !strings.Contains(taken.Error(), instance) {
		t.Errorf("the message does not say what is held: %q", taken.Error())
	}
}

// Releasing has to give the name back, or a restart would never come up.
func TestReleasingFreesTheName(t *testing.T) {
	done, err := Only()
	if err != nil {
		t.Skipf("something already holds %s: %v", instance, err)
	}
	done()

	again, err := Only()
	if err != nil {
		t.Fatalf("the name was not given back: %v", err)
	}
	again()
}
