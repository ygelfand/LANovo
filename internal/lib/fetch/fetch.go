// Package fetch is the HTTP client everything on the device downloads through.
//
// One client, so one decision about certificates applies everywhere rather than each caller
// building its own and some of them being missed.
//
// The decision it carries: this device boots in 1970 and cannot write its RTC, so a certificate
// cannot be verified until the clock is set — and the clock is often set by Home Assistant over
// the API, which is not up before onboarding. A device on a network with no route out never gets a
// valid time at all. Verifying is right, and a device that cannot verify should be able to say so
// and carry on deliberately rather than failing in a way nobody can diagnose from the front of it.
package fetch

import (
	"crypto/tls"
	"net/http"
	"sync/atomic"
	"time"
)

// skip is whether certificates are being taken on trust. Off unless something says otherwise, and
// read per request so the switch takes effect on the next download rather than the next restart.
var skip atomic.Bool

// Verify says whether certificates are checked. False is the unusual case and is meant to be
// visible: see Skipping.
func Verify(on bool) { skip.Store(!on) }

// Skipping reports whether certificates are being taken on trust, for anything that wants to say
// so where it can be seen.
func Skipping() bool { return skip.Load() }

// Client downloads with the device's certificate policy, whatever it is at the time of the
// request.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: shared}
}

// shared is one transport, so connections are pooled across everything that downloads.
var shared http.RoundTripper = &policy{
	strict: http.DefaultTransport,
	lax:    lax(),
}

// policy picks a transport per request rather than rebuilding one when the setting changes, which
// keeps each pool warm and means a download in flight is not disturbed by a switch being flipped.
type policy struct {
	strict, lax http.RoundTripper
}

func (p *policy) RoundTrip(r *http.Request) (*http.Response, error) {
	if skip.Load() {
		return p.lax.RoundTrip(r)
	}
	return p.strict.RoundTrip(r)
}

// lax is the same transport with the certificate check turned off. Its own, rather than mutating
// the default's TLS config, because http.DefaultTransport is shared with anything else in the
// process that reaches for it.
func lax() http.RoundTripper {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}

	c := t.Clone()
	c.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // the point of it
	return c
}
