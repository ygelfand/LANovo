package takeover

import (
	"errors"
	"slices"
	"testing"
	"time"
)

type remountShell struct {
	calls    []string
	failures []error
	mounts   string
}

func (s *remountShell) Shell(cmd string) (string, error) {
	s.calls = append(s.calls, cmd)
	if cmd == "mount -o remount,ro /" && len(s.failures) != 0 {
		err := s.failures[0]
		s.failures = s.failures[1:]
		return "", err
	}
	if cmd == "cat /proc/mounts" {
		return s.mounts, nil
	}
	return "", nil
}

func TestRestoreReadOnly(t *testing.T) {
	const remount = "mount -o remount,ro /"
	const part = "/dev/block/by-name/system_a"
	busy := errors.New("mount: '/dev/root'->'/': Device or resource busy")
	denied := errors.New("Operation not permitted")
	for _, tc := range []struct {
		name     string
		failures []error
		options  string
		want     []string
		wantErr  error
	}{
		{"retry succeeds", []error{busy}, "ro,relatime", []string{"sync", remount, remount, "cat /proc/mounts", "blockdev --setro " + part}, nil},
		{"busy requires reboot", []error{busy, busy, busy}, "rw,relatime", []string{"sync", remount, remount, remount}, errRemountBusy},
		{"permission denied", []error{denied}, "rw", []string{"sync", remount}, denied},
		{"verify mount before changing block flags", nil, "rw", []string{"sync", remount, "cat /proc/mounts"}, errors.New("/ is still mounted read-write")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &remountShell{failures: tc.failures, mounts: "/dev/root / ext4 " + tc.options + " 0 0\n"}
			err := restoreReadOnly(d, "/", part, true, func(time.Duration) {})
			if tc.wantErr == nil && err != nil ||
				tc.wantErr != nil && (err == nil || (!errors.Is(err, tc.wantErr) && err.Error() != tc.wantErr.Error())) {
				t.Fatalf("restoreReadOnly error = %v, want %v", err, tc.wantErr)
			}
			if !slices.Equal(d.calls, tc.want) {
				t.Fatalf("calls = %q, want %q", d.calls, tc.want)
			}
		})
	}
}

func TestRestoreReadOnlyLegacyAttempt(t *testing.T) {
	const remount = "mount -o remount,ro /"
	const part = "/dev/block/by-name/system_a"
	busy := errors.New("Device or resource busy")
	d := &remountShell{failures: []error{busy}}
	err := restoreReadOnly(d, "/", part, false, func(time.Duration) {
		t.Fatal("legacy remount retried")
	})
	if !errors.Is(err, busy) || errors.Is(err, errRemountBusy) {
		t.Fatalf("legacy remount error = %v", err)
	}
	if !slices.Equal(d.calls, []string{"sync", remount}) {
		t.Fatalf("legacy remount calls = %q", d.calls)
	}
}
