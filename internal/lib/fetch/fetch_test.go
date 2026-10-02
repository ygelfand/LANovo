package fetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Back to verifying however a test ends, since the setting is process-wide.
func strict(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { Verify(true) })
}

// A device nobody has told otherwise checks certificates.
func TestVerifyingByDefault(t *testing.T) {
	strict(t)

	if Skipping() {
		t.Error("certificates are being taken on trust before anything asked for that")
	}
}

// The real test: a server with a certificate nothing can chain to. Verifying has to refuse it and
// not verifying has to take it, because a flag that flips without changing what the transport does
// is a switch that does nothing.
func TestTheSwitchDecidesWhetherABadCertificateIsTaken(t *testing.T) {
	strict(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	Verify(true)
	if _, err := Client(5 * time.Second).Get(srv.URL); err == nil {
		t.Error("a certificate that chains to nothing was accepted while verifying")
	}

	Verify(false)
	resp, err := Client(5 * time.Second).Get(srv.URL)
	if err != nil {
		t.Fatalf("with verification off the download still failed: %v", err)
	}
	resp.Body.Close()

	// And back, in the same process: the switch is read per request, so turning it on again has to
	// take effect without a restart.
	Verify(true)
	if _, err := Client(5 * time.Second).Get(srv.URL); err == nil {
		t.Error("turning verification back on did not take effect")
	}
}

func TestSkippingSaysWhichWayItIs(t *testing.T) {
	strict(t)

	Verify(false)
	if !Skipping() {
		t.Error("Skipping is false with verification off")
	}

	Verify(true)
	if Skipping() {
		t.Error("Skipping is true with verification on")
	}
}

// Plain HTTP is unaffected either way, which is most of what this device downloads.
func TestPlainHTTPIsUntouched(t *testing.T) {
	strict(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	for _, on := range []bool{true, false} {
		Verify(on)

		resp, err := Client(5 * time.Second).Get(srv.URL)
		if err != nil {
			t.Fatalf("verify=%v: %v", on, err)
		}
		resp.Body.Close()
	}
}

// The lax transport is its own, or turning verification off would quietly stop anything else in
// the process from checking certificates too.
func TestTheDefaultTransportIsNotChanged(t *testing.T) {
	strict(t)
	Verify(false)

	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Skip("the default transport is not an *http.Transport here")
	}
	if base.TLSClientConfig != nil && base.TLSClientConfig.InsecureSkipVerify {
		t.Error("turning verification off reached into http.DefaultTransport")
	}
}
