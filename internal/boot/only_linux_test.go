package boot

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// Off Linux, net.Listen treats "@name" as a filename.

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

	if taken.Pid != os.Getpid() {
		t.Errorf("the holder is pid %d, want this process at %d", taken.Pid, os.Getpid())
	}
	if !strings.Contains(taken.Error(), instance) {
		t.Errorf("the message does not say what is held: %q", taken.Error())
	}
}

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
