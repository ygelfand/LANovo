package boot

import (
	"strings"
	"testing"
)

const table = `Num       RefCount Protocol Flags    Type St Inode Path
00000000: 00000002 00000000 00010000 0005 01 18691 /dev/socket/lmkd
00000000: 00000002 00000000 00000000 0002 01 120751 @lanovod-4943-4
00000000: 00000002 00000000 00000000 0002 01 120749 @lanovod-4943-3
00000000: 00000002 00000000 00010000 0001 01 125021 @lanovod-instance
00000000: 00000002 00000000 00010000 0001 01 120780 @lanovod
00000000: 00000003 00000000 00000000 0001 03 12345
`

func TestTheInodeIsReadOffTheTable(t *testing.T) {
	got, ok := inodeIn(strings.NewReader(table), instance)
	if !ok {
		t.Fatal("the lock is in the table and was not found")
	}
	if got != "125021" {
		t.Errorf("inode %q, want 125021", got)
	}
}

func TestOnlyTheExactNameMatches(t *testing.T) {
	for _, name := range []string{"@lanovod", "@lanovod-4943-3"} {
		got, ok := inodeIn(strings.NewReader(table), name)
		if !ok {
			t.Fatalf("%s is in the table and was not found", name)
		}
		if got == "125021" {
			t.Errorf("%s resolved to the lock's inode", name)
		}
	}
}

func TestANameThatIsNotThereIsNotFound(t *testing.T) {
	if _, ok := inodeIn(strings.NewReader(table), "@nothing-like-it"); ok {
		t.Error("a name that is not in the table was found")
	}
}

// A socket with no path has no name column in /proc/net/unix.
func TestALineWithNoPathIsSkipped(t *testing.T) {
	if _, ok := inodeIn(strings.NewReader(table), ""); ok {
		t.Error("an empty name matched a line with no path")
	}
}

func TestRubbishIsNotAnInode(t *testing.T) {
	const bad = "00000000: 00000002 00000000 00010000 0001 01 notanumber @lanovod-instance\n"

	if _, ok := inodeIn(strings.NewReader(bad), instance); ok {
		t.Error("a line whose inode is not a number was taken")
	}
}

func TestTheExitCodeIsItsOwn(t *testing.T) {
	if ExitTaken == 0 || ExitTaken == 1 || ExitTaken == 2 {
		t.Errorf(
			"ExitTaken is %d, which is not distinguishable from an ordinary failure",
			ExitTaken,
		)
	}
}

func TestTheLockIsAbstract(t *testing.T) {
	if !strings.HasPrefix(instance, "@") {
		t.Errorf("%q is not an abstract name, so it leaves a file behind", instance)
	}
}

func TestAnUnknownHolderStillSaysWhatIsHeld(t *testing.T) {
	msg := (&Taken{Err: errNowhere}).Error()

	if !strings.Contains(msg, instance) {
		t.Errorf("the message does not name the socket: %q", msg)
	}
	if !strings.Contains(msg, errNowhere.Error()) {
		t.Errorf("the message drops what the kernel said: %q", msg)
	}
}

func TestANamedHolderIsInTheMessage(t *testing.T) {
	msg := (&Taken{Pid: 4943, Name: "lanovod", Err: errNowhere}).Error()

	for _, want := range []string{"4943", "lanovod", instance} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message leaves out %q: %q", want, msg)
		}
	}
}

type nowhere struct{}

func (nowhere) Error() string { return "bind: address already in use" }

var errNowhere = nowhere{}
