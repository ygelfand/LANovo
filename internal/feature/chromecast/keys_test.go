package chromecast

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
)

func testKeys(t *testing.T) *keys {
	t.Helper()
	k, err := newKeys("Kitchen", filepath.Join(t.TempDir(), "cast-credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func oracleAnswer(t *testing.T, k *keys, from, until, now time.Time) []byte {
	t.Helper()
	c, err := k.authority.Issue("Kitchen", from.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(c.Peer.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString
	body, _ := json.Marshal(cast.Oracle{
		Public:       b64(c.Peer.Certificate[0]),
		Private:      b64(key),
		Device:       b64(c.Device),
		Intermediate: b64(c.Intermediates[0]),
		SHA1:         b64(c.SHA1),
		SHA256:       b64(c.SHA256),
		NotBefore:    from.Unix(),
		NotAfter:     until.Unix(),
		Now:          now.Unix(),
	})
	return body
}

func serve(t *testing.T, body []byte) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(body) }))
	t.Cleanup(s.Close)
	return s.URL
}

func TestWithNoOracleTheCredentialsAreOurOwn(t *testing.T) {
	k := testKeys(t)
	wait := k.step(t.Context(), "")

	if c := k.Current(); c == nil || c.Remote {
		t.Fatalf("the credentials are %+v", c)
	}
	if wait <= 0 || wait > cast.PeerLife {
		t.Errorf("the next look is in %v", wait)
	}
}

func TestAnOracleAnswerIsUsedKeptAndReadBack(t *testing.T) {
	k := testKeys(t)
	now := time.Now()
	url := serve(t, oracleAnswer(t, k, now.Add(-time.Hour), now.Add(47*time.Hour), now))

	wait := k.step(t.Context(), url)
	c := k.Current()
	if !c.Remote {
		t.Fatal("the oracle's answer was not used")
	}
	if wait < 46*time.Hour || wait > 47*time.Hour {
		t.Errorf("the next look is in %v, want at notAfter", wait)
	}
	if again := k.step(t.Context(), url); again < 46*time.Hour {
		t.Errorf("a current answer was asked for again, next look in %v", again)
	}

	if kept, from := k.load(url); kept == nil || from != url || kept.NotAfter != c.NotAfter {
		t.Error("the answer was not kept")
	}
	if kept, _ := k.load("http://elsewhere"); kept != nil {
		t.Error("an answer kept from one oracle was offered for another")
	}
}

func TestACertificateNotCurrentYetIsRetried(t *testing.T) {
	k := testKeys(t)
	now := time.Now()
	url := serve(t, oracleAnswer(t, k, now.Add(time.Hour), now.Add(49*time.Hour), now))

	if wait := k.step(t.Context(), url); wait != Retry {
		t.Errorf("the next look is in %v, want %v", wait, Retry)
	}
	if k.Current().Remote {
		t.Error("a certificate not valid yet was used")
	}
}

func TestTheRevocationListIsFetchedAndHeld(t *testing.T) {
	k := testKeys(t)
	k.crlURL = serve(t, []byte("revoked"))

	if k.CRL() != nil {
		t.Fatal("a revocation list before any was fetched")
	}

	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	go k.revocations(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for k.CRL() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := string(k.CRL()); got != "revoked" {
		t.Errorf("the revocation list is %q", got)
	}
}

func TestClearingTheOracleGoesBackToOurOwn(t *testing.T) {
	k := testKeys(t)
	now := time.Now()
	k.step(t.Context(), serve(t, oracleAnswer(t, k, now.Add(-time.Hour), now.Add(47*time.Hour), now)))

	k.step(t.Context(), "")
	if k.Current().Remote {
		t.Error("the oracle's credentials stayed after the oracle was cleared")
	}
}

func TestTheDeviceChainIsMadeOnceAndKept(t *testing.T) {
	dir := t.TempDir()
	creds := filepath.Join(dir, "cast-credentials.json")

	first, err := newKeys("Kitchen", creds)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cast-authority.json")); err != nil {
		t.Fatalf("the chain was not kept: %v", err)
	}
	again, err := newKeys("Kitchen", creds)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Current().Device, again.Current().Device) {
		t.Error("a second start made a new device certificate instead of reading the kept one")
	}

	renamed, err := newKeys("Den", creds)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.Current().Device, renamed.Current().Device) {
		t.Error("a device given a new name kept the chain made for the old one")
	}
}

func TestAKeptChainThatDoesNotReadIsMadeAgain(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cast-authority.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	k, err := newKeys("Kitchen", filepath.Join(dir, "cast-credentials.json"))
	if err != nil || k.Current() == nil {
		t.Fatalf("a damaged kept chain stopped the keys: %v", err)
	}
}
