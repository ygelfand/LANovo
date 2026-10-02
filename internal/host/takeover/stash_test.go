package takeover

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/layout"
)

func TestEveryAppIsStashedUnderItsOwnFolder(t *testing.T) {
	got := moves("/system/app/webview\n/system/priv-app/Shell\n/system/app/KeyChain\n")
	want := []move{
		{"/system/app/webview", layout.Stash + "/app/webview"},
		{"/system/priv-app/Shell", layout.Stash + "/priv-app/Shell"},
		{"/system/app/KeyChain", layout.Stash + "/app/KeyChain"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("move %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestNothingOutsideTheAppFoldersIsStashed(t *testing.T) {
	for _, listing := range []string{
		"", "/system/app/*\n/system/priv-app/*", "/system/framework/framework.jar", "/system/app", "/system/bin/lanovod",
	} {
		if got := moves(listing); len(got) != 0 {
			t.Errorf("%q stashed %v", listing, got)
		}
	}
}
