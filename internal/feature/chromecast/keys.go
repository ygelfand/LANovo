package chromecast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/libcountertop/pkg/fetch"
	"github.com/ygelfand/libcountertop/pkg/media/cast"
)

// Retry is how long to wait after an oracle answer that could not be used.
const Retry = 30 * time.Second

// Drift is how far the local clock may be from the oracle's before it is logged.
const Drift = 5 * time.Minute

// CRLURL is Google's Cast device revocation list, an opaque protobuf that stock devices send with
// every auth answer.
const CRLURL = "https://clients3.google.com/cast/chromecast/device/crl"

// CRLEvery is how often the revocation list is fetched again once there is one.
const CRLEvery = 24 * time.Hour

// keys holds the current device credentials and replaces them at notAfter.
type keys struct {
	name      string
	path      string
	authority *cast.Authority

	held     atomic.Pointer[held]
	crl      atomic.Pointer[[]byte]
	crlURL   string
	retarget chan struct{}
	changed  func(*cast.Credentials)
}

type held struct {
	creds *cast.Credentials
	from  string
}

type cached struct {
	URL    string          `json:"url"`
	Answer json.RawMessage `json:"answer"`
}

func newKeys(name, path string) (*keys, error) {
	a, err := authority(name, filepath.Join(filepath.Dir(path), filepath.Base(layout.CastAuthorityPath)))
	if err != nil {
		return nil, err
	}
	k := &keys{name: name, path: path, authority: a, crlURL: CRLURL, retarget: make(chan struct{}, 1)}
	if err := k.issue(time.Now()); err != nil {
		return nil, err
	}
	return k, nil
}

// authority is the chain kept at path, or a new one made and kept there when there is none for
// this name.
func authority(name, path string) (*cast.Authority, error) {
	if raw, err := os.ReadFile(path); err == nil {
		a, err := cast.LoadAuthority(raw, name)
		if err == nil {
			return a, nil
		}
		slog.Warn("making a new cast authority", "err", err)
	}
	a, err := cast.NewAuthority(name)
	if err != nil {
		return nil, err
	}
	data, err := a.Marshal()
	if err == nil {
		err = writeKept(path, data)
	}
	if err != nil {
		slog.Warn("keeping the cast authority", "err", err)
	}
	return a, nil
}

func writeKept(path string, data []byte) error {
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Current is the credentials to present and answer with now.
func (k *keys) Current() *cast.Credentials {
	if h := k.held.Load(); h != nil {
		return h.creds
	}
	return nil
}

// CRL is the revocation list to answer with, or nil until one has been fetched.
func (k *keys) CRL() []byte {
	if c := k.crl.Load(); c != nil {
		return *c
	}
	return nil
}

// revocations fetches the revocation list, again after Retry while it fails and after CRLEvery once
// it has one.
func (k *keys) revocations(ctx context.Context) {
	for {
		wait := CRLEvery
		body, err := ask(ctx, k.crlURL)
		switch {
		case err != nil:
			slog.Warn("fetching the cast revocation list", "err", err)
			wait = Retry
		case len(body) == 0:
			slog.Warn("the cast revocation list came back empty")
			wait = Retry
		default:
			k.crl.Store(&body)
			slog.Debug("cast revocation list", "bytes", len(body))
		}

		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// Retarget is the oracle setting having changed.
func (k *keys) Retarget() {
	select {
	case k.retarget <- struct{}{}:
	default:
	}
}

func (k *keys) set(c *cast.Credentials, from string) {
	k.held.Store(&held{creds: c, from: from})
	slog.Info("cast credentials", "remote", c.Remote, "notBefore", c.NotBefore, "notAfter", c.NotAfter)
	if k.changed != nil {
		k.changed(c)
	}
}

func (k *keys) issue(now time.Time) error {
	c, err := k.authority.Issue(k.name, now)
	if err != nil {
		return err
	}
	k.set(c, "")
	return nil
}

func (k *keys) run(ctx context.Context) {
	synced := make(chan struct{}, 1)
	defer clock.Get().Synced.Listen(func(time.Time) {
		select {
		case synced <- struct{}{}:
		default:
		}
	})()

	if c, from := k.load(config.Get().Cast.Oracle); c != nil {
		k.set(c, from)
	}
	go k.revocations(ctx)

	for {
		t := time.NewTimer(k.step(ctx, config.Get().Cast.Oracle))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		case <-k.retarget:
			t.Stop()
		case <-synced:
			t.Stop()
		}
	}
}

// step brings the credentials up to date and says how long until they next need looking at.
func (k *keys) step(ctx context.Context, url string) time.Duration {
	now := time.Now()
	h := k.held.Load()

	if url == "" {
		if h.creds.Remote || !now.Before(h.creds.NotAfter) {
			if err := k.issue(now); err != nil {
				slog.Warn("making cast credentials", "err", err)
				return Retry
			}
			h = k.held.Load()
		}
		return time.Until(h.creds.NotAfter)
	}

	if h.creds.Remote && h.from == url && now.Before(h.creds.NotAfter) {
		return time.Until(h.creds.NotAfter)
	}
	if !h.creds.Remote && !now.Before(h.creds.NotAfter) {
		if err := k.issue(now); err != nil {
			slog.Warn("making cast credentials", "err", err)
		}
	}

	body, err := ask(ctx, url)
	if err != nil {
		slog.Warn("asking the cast oracle", "err", err)
		return Retry
	}
	c, theirs, err := cast.ParseOracle(body)
	if err != nil {
		slog.Warn("reading the cast oracle", "err", err)
		return Retry
	}

	if d := now.Sub(theirs); d > Drift || d < -Drift {
		slog.Warn("the clock disagrees with the cast oracle", "ours", now, "theirs", theirs)
	}
	if c.NotBefore.After(theirs) || !theirs.Before(c.NotAfter) {
		slog.Warn("the cast oracle's certificate is not current",
			"notBefore", c.NotBefore, "notAfter", c.NotAfter, "now", theirs)
		return Retry
	}

	k.set(c, url)
	if err := k.save(url, body); err != nil {
		slog.Warn("keeping the cast oracle's answer", "err", err)
	}
	return c.NotAfter.Sub(theirs)
}

func ask(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := fetch.Client(30 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// load is the kept answer, if it came from the oracle now set.
func (k *keys) load(url string) (*cast.Credentials, string) {
	if url == "" {
		return nil, ""
	}
	raw, err := os.ReadFile(k.path)
	if err != nil {
		return nil, ""
	}
	var c cached
	if err := json.Unmarshal(raw, &c); err != nil || c.URL != url {
		return nil, ""
	}
	creds, _, err := cast.ParseOracle(c.Answer)
	if err != nil {
		slog.Warn("reading the kept cast oracle answer", "err", err)
		return nil, ""
	}
	return creds, url
}

func (k *keys) save(url string, answer []byte) error {
	data, err := json.Marshal(cached{URL: url, Answer: answer})
	if err != nil {
		return err
	}

	f, err := os.CreateTemp(filepath.Dir(k.path), filepath.Base(k.path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, k.path)
}

type kept struct{ path string }

func keep(app string) cast.Kept { return kept{path: filepath.Join(layout.CastAppDir, app+".json")} }

func (k kept) Load(v any) bool {
	data, err := os.ReadFile(k.path)
	return err == nil && json.Unmarshal(data, v) == nil
}

func (k kept) Save(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(k.path), 0o700); err != nil {
		return err
	}
	return writeKept(k.path, data)
}

func (k kept) Clear() error {
	if err := os.Remove(k.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
