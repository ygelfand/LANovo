// Package youtube is YouTube's receiver protocol: MDX on the cast channel, which hands the sender a
// lounge screen, and the lounge itself, where the phone sends its queue through YouTube's servers.
package youtube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ygelfand/LANovo/internal/lib/cast"
)

// The namespaces a running YouTube receiver reports.
const (
	NSMDX          = "urn:x-cast:com.google.youtube.mdx"
	NSCAC          = "urn:x-cast:com.google.cast.cac"
	NSDebugOverlay = "urn:x-cast:com.google.cast.debugoverlay"
)

// Name is what the app table calls this protocol.
const Name = "youtube"

const (
	typeGetStatus = "getMdxSessionStatus"
	typeStatus    = "mdxSessionStatus"
)

// Protocol answers the cast side of YouTube for the applications that name it, and keeps the
// lounge screens they pair through.
type Protocol struct {
	screens func(app string) (screen, device string, ok bool)
	keeper  *screens
}

func init() {
	cast.Define(Name, func(env cast.Env) cast.Protocol { return New(env) })
}

// New is the protocol, with its screens loaded from the config.
func New(env cast.Env) *Protocol {
	k := newScreens(env)
	return &Protocol{screens: k.screen, keeper: k}
}

// Run keeps the lounge screens bound until ctx ends.
func (p *Protocol) Run(ctx context.Context) {
	if p.keeper != nil {
		p.keeper.run(ctx)
	}
}

func (p *Protocol) Name() string { return Name }

func (p *Protocol) Namespaces() []string { return []string{NSMDX, NSCAC, NSDebugOverlay} }

type sessionStatus struct {
	Type string     `json:"type"`
	Data statusData `json:"data"`
}

type statusData struct {
	ScreenID string `json:"screenId"`
	DeviceID string `json:"deviceId"`
}

// Receive answers getMdxSessionStatus with the application's lounge screen.
func (p *Protocol) Receive(app *cast.Application, m cast.Message) ([]cast.Message, error) {
	if m.Namespace != NSMDX {
		return nil, cast.ErrUnspoken
	}
	var h struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(m.Payload), &h); err != nil {
		return nil, fmt.Errorf("youtube: an mdx message: %w", err)
	}
	if h.Type != typeGetStatus {
		return nil, cast.ErrUnspoken
	}

	if p.keeper != nil {
		p.keeper.wake(app.AppID)
	}
	screen, device, ok := p.screens(app.AppID)
	if !ok {
		return nil, cast.ErrUnspoken
	}
	body, err := json.Marshal(sessionStatus{Type: typeStatus, Data: statusData{ScreenID: screen, DeviceID: device}})
	if err != nil {
		return nil, err
	}
	return []cast.Message{cast.Reply(m, NSMDX, string(body))}, nil
}
