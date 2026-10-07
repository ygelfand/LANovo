package discovery

import (
	"context"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/zeroconf/v2"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	mdns "github.com/ygelfand/libcountertop/pkg/network/advertise"
	sharedpeer "github.com/ygelfand/libcountertop/pkg/network/peer"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
	"github.com/ygelfand/libcountertop/pkg/say"
)

func init() {
	component.Register(component.Network, Get, component.Order(60))
	dashboard.AddTabs(100, func() []dashboard.Tab {
		return []dashboard.Tab{{Kind: TabKind, Key: TabKind, Name: say.T("call.tab")}}
	})
}

const TabKind = "call"

const (
	service = "_countertop._tcp"
	domain  = "local."
	proto   = "1"
	ttl     = 120
	listen  = 4 * time.Second
	every   = 30 * time.Second
	missed  = 6
)

type Peer = sharedpeer.Peer

type Discovery struct {
	mu    sync.Mutex
	peers map[string]Peer
}

var (
	once   sync.Once
	shared *Discovery
)

func Get() *Discovery {
	once.Do(func() { shared = &Discovery{peers: map[string]Peer{}} })
	return shared
}

func (d *Discovery) Name() string { return "discovery" }

// Self describes this device using its current identity and addresses.
func (d *Discovery) Self() Peer { return Self() }

func (d *Discovery) Peers() []Peer {
	d.mu.Lock()
	out := make([]Peer, 0, len(d.peers))
	for _, p := range d.peers {
		out = append(out, p)
	}
	d.mu.Unlock()
	slices.SortFunc(
		out,
		func(a, b Peer) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) },
	)
	return out
}

func (d *Discovery) Touch(id string) {
	d.mu.Lock()
	if p, ok := d.peers[id]; ok {
		p.Seen = time.Now()
		d.peers[id] = p
	}
	d.mu.Unlock()
}

func Self() Peer {
	b := board.Current()
	return Peer{
		ID:    ID(),
		Name:  config.Get().Device.Name,
		Model: b.Model,
		Board: b.Name,
		Caps:  caps(),
	}
}

func (d *Discovery) Find(id string) (Peer, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, ok := d.peers[id]
	return p, ok
}

func (d *Discovery) Run(ctx context.Context) error {
	self := ID()
	safe.Go("discovery advertise", func() { advertise(ctx, self) })
	for {
		d.sweep(ctx, self)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(every):
		}
	}
}

func ID() string { return strings.ToLower(strings.ReplaceAll(wifi.Get().MAC(), ":", "")) }

func caps() []string {
	c := []string{"audio"}
	if board.Current().CameraWidth > 0 {
		c = append(c, "video")
	}
	return c
}

func record(self string) []string {
	b := board.Current()
	return []string{
		"id=" + self,
		"name=" + config.Get().Device.Name,
		"model=" + b.Model,
		"board=" + b.Name,
		"project=" + strings.ToLower(layout.Manufacturer),
		"version=" + layout.Version,
		"proto=" + proto,
		"caps=" + strings.Join(caps(), ","),
	}
}

func advertise(ctx context.Context, self string) {
	mdns.Advertise(ctx, "countertop", func(ips []net.IP) (func(), error) {
		name := config.Get().Device.Name
		srv, err := zeroconf.RegisterProxy(
			name, service, domain, web.Port, layout.Slug(name), mdns.Strings(ips),
			record(self), nil, zeroconf.TTL(ttl),
		)
		if err != nil {
			return nil, err
		}
		return srv.Shutdown, nil
	})
}

func (d *Discovery) sweep(ctx context.Context, self string) {
	ctx, cancel := context.WithTimeout(ctx, listen)
	defer cancel()
	entries := make(chan *zeroconf.ServiceEntry, 8)
	safe.Go("discovery browse", func() {
		if err := zeroconf.Browse(ctx, service, domain, entries); err != nil {
			slog.Debug("countertop browse", "err", err)
		}
	})
	now := time.Now()
	for e := range entries {
		p := parse(e, now)
		if p.ID == "" || p.ID == self {
			continue
		}
		d.mu.Lock()
		_, known := d.peers[p.ID]
		d.peers[p.ID] = p
		d.mu.Unlock()
		if !known {
			slog.Info("peer found", "name", p.Name, "model", p.Model, "addrs", p.Addrs)
			shell.Get().Redraw()
		}
	}
	d.expire(now)
}

func (d *Discovery) expire(now time.Time) {
	gone := now.Add(-missed * every)
	d.mu.Lock()
	var lost []Peer
	for k, p := range d.peers {
		if p.Seen.Before(gone) {
			lost = append(lost, p)
			delete(d.peers, k)
		}
	}
	d.mu.Unlock()
	for _, p := range lost {
		slog.Info("peer lost", "name", p.Name)
	}
	if len(lost) > 0 {
		shell.Get().Redraw()
	}
}

func parse(e *zeroconf.ServiceEntry, now time.Time) Peer {
	txt := map[string]string{}
	for _, kv := range e.Text {
		k, v, _ := strings.Cut(kv, "=")
		txt[strings.ToLower(k)] = v
	}
	p := Peer{
		ID:      txt["id"],
		Name:    txt["name"],
		Model:   txt["model"],
		Board:   txt["board"],
		Project: txt["project"],
		Version: txt["version"],
		Host:    strings.TrimSuffix(e.HostName, "."),
		Addrs:   append(append([]net.IP(nil), e.AddrIPv4...), e.AddrIPv6...),
		Port:    e.Port,
		Seen:    now,
	}
	if c := txt["caps"]; c != "" {
		p.Caps = strings.Split(c, ",")
	}
	if p.Name == "" {
		p.Name = e.Instance
	}
	return p
}
