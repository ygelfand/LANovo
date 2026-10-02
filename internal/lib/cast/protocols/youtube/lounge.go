package youtube

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Base is where the lounge lives.
const Base = "https://www.youtube.com/api/lounge"

// The themes a screen binds with: YouTube's own, YouTube Music's and YouTube TV's.
const (
	ThemeYouTube = "cl"
	ThemeMusic   = "m"
	ThemeTV      = "up"
)

// What a Chromecast calls itself on the lounge, from a captured session.
const (
	ScreenApp          = "lb-v4"
	ScreenCapabilities = "dsp,que,mus"
	ScreenClient       = "TVHTML5"
)

// ErrExpired is a lounge answer that means the token or the session has to be renewed.
var ErrExpired = errors.New("lounge: the session has expired")

var ErrQuiet = errors.New("lounge: the long poll went quiet")

// closure-library's BrowserChannel expects server data within 45s.
var Quiet = 45 * time.Second

// Screen is this device as a lounge screen.
type Screen struct {
	ID     string
	Device string
	Name   string
	Theme  string
	Brand  string
	Model  string
}

// Token is what binding needs, and when it stops working.
type Token struct {
	Screen  string
	Token   string
	Expires time.Time
}

// NewScreenID asks YouTube for a screen id.
func NewScreenID(ctx context.Context, c *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, Base+"/pairing/generate_screen_id", nil)
	if err != nil {
		return "", err
	}
	body, err := do(c, req)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(body))
	if id == "" {
		return "", fmt.Errorf("lounge: an empty screen id")
	}
	return id, nil
}

// LoungeToken asks YouTube for the token a screen binds with.
func LoungeToken(ctx context.Context, c *http.Client, screen string) (Token, error) {
	form := url.Values{"screen_ids": {screen}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Base+"/pairing/get_lounge_token_batch",
		strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := do(c, req)
	if err != nil {
		return Token{}, err
	}
	var got struct {
		Screens []struct {
			ScreenID    string `json:"screenId"`
			LoungeToken string `json:"loungeToken"`
			Expiration  int64  `json:"expiration"`
		} `json:"screens"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		return Token{}, fmt.Errorf("lounge: the token answer: %w", err)
	}
	if len(got.Screens) == 0 || got.Screens[0].LoungeToken == "" {
		return Token{}, fmt.Errorf("lounge: no token for screen %s", screen)
	}
	s := got.Screens[0]
	return Token{Screen: s.ScreenID, Token: s.LoungeToken, Expires: time.UnixMilli(s.Expiration)}, nil
}

// Session is one bound lounge session.
type Session struct {
	screen Screen
	token  Token
	short  *http.Client
	long   *http.Client

	mu  sync.Mutex
	sid string
	gid string
	rid int
	aid int
	ofs int
}

// Bind opens a session as the screen. short is for ordinary requests; long must not time out,
// since it carries the long poll.
func Bind(ctx context.Context, short, long *http.Client, screen Screen, token Token) (*Session, error) {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	s := &Session{screen: screen, token: token, short: short, long: long, rid: 41000 + int(n.Int64()), aid: 3}

	q := s.common()
	q.Set("deviceInfo", s.deviceInfo())
	q.Set("RID", fmt.Sprint(s.rid))
	q.Set("CVER", "1")
	s.rid++

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Base+"/bc/bind?"+q.Encode(),
		strings.NewReader(url.Values{"count": {"0"}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := do(short, req)
	if err != nil {
		return nil, err
	}
	msgs, err := Parse(string(body))
	if err != nil {
		return nil, err
	}
	s.take(msgs)
	if s.sid == "" || s.gid == "" {
		return nil, fmt.Errorf("lounge: the bind answer has no session")
	}
	return s, nil
}

// Poll holds the long poll open, handing each message over, until it ends or ctx does.
func (s *Session) Poll(ctx context.Context, got func(Message)) error {
	s.mu.Lock()
	q := s.common()
	q.Set("RID", "rpc")
	q.Set("SID", s.sid)
	q.Set("CI", "0")
	q.Set("AID", fmt.Sprint(s.aid))
	q.Set("gsessionid", s.gid)
	q.Set("TYPE", "xmlhttp")
	s.mu.Unlock()

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	watch := time.AfterFunc(Quiet, func() { cancel(ErrQuiet) })
	defer watch.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, Base+"/bc/bind?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := s.long.Do(req)
	if err != nil {
		return quiet(ctx, err)
	}
	defer resp.Body.Close()
	if err := status(resp); err != nil {
		return err
	}

	err = Frames(resp.Body, func(msgs []Message) {
		watch.Reset(Quiet)
		s.take(msgs)
		for _, m := range msgs {
			got(m)
		}
	})
	return quiet(ctx, err)
}

func quiet(ctx context.Context, err error) error {
	if errors.Is(context.Cause(ctx), ErrQuiet) {
		return ErrQuiet
	}
	return err
}

// Send posts messages to the lounge.
func (s *Session) Send(ctx context.Context, msgs ...Out) error {
	if len(msgs) == 0 {
		return nil
	}

	s.mu.Lock()
	q := s.common()
	q.Set("deviceInfo", s.deviceInfo())
	q.Set("SID", s.sid)
	q.Set("RID", fmt.Sprint(s.rid))
	q.Set("AID", fmt.Sprint(s.aid))
	q.Set("gsessionid", s.gid)
	s.rid++
	body := form(s.ofs, msgs)
	s.ofs += len(msgs)
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Base+"/bc/bind?"+q.Encode(),
		strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = do(s.short, req)
	return err
}

// take keeps what the session learns from messages: its ids and the highest array id seen.
func (s *Session) take(msgs []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range msgs {
		switch m.Name {
		case "c":
			s.sid = m.First()
		case "S":
			s.gid = m.String()
		}
		if m.AID > s.aid {
			s.aid = m.AID
		}
	}
}

func (s *Session) common() url.Values {
	zx := make([]byte, 6)
	rand.Read(zx)
	return url.Values{
		"device":           {"LOUNGE_SCREEN"},
		"id":               {s.screen.Device},
		"obfuscatedGaiaId": {""},
		"name":             {s.screen.Name},
		"app":              {ScreenApp},
		"theme":            {s.screen.Theme},
		"capabilities":     {ScreenCapabilities},
		"cst":              {"m"},
		"mdxVersion":       {"2"},
		"loungeIdToken":    {s.token.Token},
		"VER":              {"8"},
		"v":                {"2"},
		"zx":               {hex.EncodeToString(zx)},
		"t":                {"1"},
	}
}

func (s *Session) deviceInfo() string {
	b, _ := json.Marshal(map[string]any{
		"brand":                          s.screen.Brand,
		"model":                          s.screen.Model,
		"year":                           0,
		"os":                             "Android",
		"osVersion":                      "",
		"chipset":                        "",
		"clientName":                     ScreenClient,
		"dialAdditionalDataSupportLevel": "unsupported",
		"mdxDialServerType":              "MDX_DIAL_SERVER_TYPE_UNKNOWN",
	})
	return string(b)
}

func do(c *http.Client, req *http.Request) ([]byte, error) {
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := status(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func status(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusBadRequest, resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusGone:
		return fmt.Errorf("%w: %s", ErrExpired, resp.Status)
	default:
		return fmt.Errorf("lounge: %s", resp.Status)
	}
}
