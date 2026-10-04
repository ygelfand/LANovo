// Package chromecast answers as a Chromecast, so anything in the house can cast to this device.
//
// The protocol is internal/lib/cast. This is the part that owns a socket: the listener on 8009, the
// service on the network, and the switch that says whether either should exist.
package chromecast

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dhcp"
	"github.com/ygelfand/LANovo/internal/feature/network"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/hardware/video"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/cast"
	_ "github.com/ygelfand/LANovo/internal/lib/cast/protocols/all"
	"github.com/ygelfand/LANovo/internal/lib/cast/protocols/youtube"
	"github.com/ygelfand/LANovo/internal/lib/fetch"
	"github.com/ygelfand/LANovo/internal/lib/safe"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	"github.com/ygelfand/LANovo/internal/ui"
)

func init() {
	component.Register(component.Device, Get, component.Order(60))
}

// Receiver is the device as something to cast to.
//
// Nothing runs until the switch is on. Off it holds no socket and answers no query, which is the
// point of the switch: a device that advertises itself on the network is not something to turn on
// for somebody.
type Receiver struct {
	enable *esphome.Switch
	demand *esphome.Switch
	skip   *esphome.Text
	delay  *esphome.Number

	mu       sync.Mutex
	service  *cast.Service
	listener net.Listener
	setup    *http.Server
	advert   *cast.Advertiser
	stop     context.CancelFunc

	loud *loudness

	boot atomic.Pointer[component.Progress]

	keys      *keys
	protocols []cast.Protocol
	oracle    *esphome.Text
	source    *esphome.TextSensor
	expires   *esphome.Sensor
}

var (
	once   sync.Once
	shared *Receiver
)

func Get() *Receiver {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Receiver {
	r := &Receiver{
		enable: &esphome.Switch{
			Base: esphome.Base{
				ObjectID: "cast_receiver",
				Name:     "Cast receiver",
				Icon:     "mdi:cast",
				Category: esphome.CategoryConfig,
			},
		},
	}

	r.oracle = &esphome.Text{
		Base: esphome.Base{
			ObjectID: "cast_oracle",
			Name:     "Cast oracle",
			Icon:     "mdi:certificate",
			Category: esphome.CategoryConfig,
		},
		MaxLength: 255,
	}
	r.source = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "cast_credentials",
			Name:     "Cast credentials",
			Icon:     "mdi:certificate",
			Category: esphome.CategoryDiagnostic,
		},
	}
	r.expires = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "cast_credentials_expire",
			Name:     "Cast credentials expire",
			Icon:     "mdi:certificate-outline",
			Category: esphome.CategoryDiagnostic,
		},
		DeviceClass: "timestamp",
	}

	r.demand = &esphome.Switch{
		Base: esphome.Base{
			ObjectID: "youtube_lounge_on_demand",
			Name:     "YouTube lounge on demand",
			Icon:     "mdi:youtube",
			Category: esphome.CategoryConfig,
		},
	}
	r.demand.Set(config.Get().Cast.YouTube.OnDemand)
	r.demand.OnCommand = r.SetLoungeOnDemand

	r.skip = &esphome.Text{
		Base: esphome.Base{
			ObjectID: "youtube_sponsorblock",
			Name:     "YouTube SponsorBlock categories",
			Icon:     "mdi:debug-step-over",
			Category: esphome.CategoryConfig,
		},
		MaxLength: 255,
	}
	r.skip.Set(strings.Join(config.Get().Cast.YouTube.Skip, ","))
	r.skip.OnCommand = r.SetSkip

	r.delay = &esphome.Number{
		Base: esphome.Base{
			ObjectID: "youtube_live_delay",
			Name:     "YouTube live delay",
			Icon:     "mdi:timer-sand",
			Category: esphome.CategoryConfig,
		},
		Min: config.LiveDelayLeast, Max: config.LiveDelayMost, Step: 1, Unit: "s",
		Mode: esphome.NumberSlider,
	}
	r.delay.Set(float32(config.Get().Cast.YouTube.LiveDelay))
	r.delay.OnCommand = func(v float32) { r.SetLiveDelay(int(v)) }

	r.enable.Set(config.Get().Cast.Receiver)
	r.enable.OnCommand = r.SetReceiver
	r.oracle.Set(config.Get().Cast.Oracle)
	r.oracle.OnCommand = r.SetOracle
	r.loud = newLoudness()

	return r
}

func (r *Receiver) Name() string { return "cast receiver" }

func (r *Receiver) output() *output {
	o := newOutput()
	o.ended = func() {
		r.mu.Lock()
		s := r.service
		r.mu.Unlock()
		if s != nil {
			s.End()
		}
	}
	return o
}

func (r *Receiver) Entities() []esphome.Entity {
	return []esphome.Entity{r.enable, r.oracle, r.source, r.expires, r.demand, r.skip, r.delay}
}

func (r *Receiver) SetSkip(text string) {
	categories := youtube.ParseCategories(text)
	if err := config.Set().Cast().Skip(categories); err != nil {
		slog.Error("saving the sponsorblock categories failed", "err", err)
		return
	}
	r.skip.Set(strings.Join(categories, ","))
}

func (r *Receiver) SetLiveDelay(seconds int) {
	if err := config.Set().Cast().LiveDelay(seconds); err != nil {
		slog.Error("saving the live delay failed", "err", err)
		return
	}
	r.delay.Set(float32(config.Get().Cast.YouTube.LiveDelay))
}

func (r *Receiver) SetLoungeOnDemand(on bool) {
	r.demand.Set(on)
	if err := config.Set().Cast().LoungeOnDemand(on); err != nil {
		slog.Error("saving the lounge setting failed", "err", err)
		return
	}
	if !r.Enabled() {
		return
	}
	r.down()
	if err := r.up(); err != nil {
		slog.Error("the cast receiver would not start", "err", err)
	}
}

// SetOracle changes where device credentials come from. Empty is credentials made on the device.
func (r *Receiver) SetOracle(url string) {
	url = strings.TrimSpace(url)
	if err := config.Set().Cast().Oracle(url); err != nil {
		slog.Error("saving the cast oracle failed", "err", err)
		return
	}
	r.oracle.Set(url)
	r.mu.Lock()
	k := r.keys
	r.mu.Unlock()
	if k != nil {
		k.Retarget()
	}
}

func (r *Receiver) credentials(c *cast.Credentials) {
	if !c.Remote {
		r.source.Set("self-signed")
		r.expires.Set(float32(math.NaN()))
		return
	}
	r.source.Set("remote-signed")
	r.expires.Set(float32(c.NotAfter.Unix()))
}

// Enabled reports whether the user has asked for it.
func (r *Receiver) Enabled() bool { return config.Get().Cast.Receiver }

// Start brings the receiver up if it is wanted, once the network is there.
func (r *Receiver) Start(context.Context) error {
	if !r.Enabled() {
		r.booted(component.Progress{Done: true, Doing: "off"})
		return nil
	}
	r.booted(component.Progress{Doing: "waiting for the network"})
	return nil
}

func (r *Receiver) Run(ctx context.Context) error {
	if !r.Enabled() {
		<-ctx.Done()
		return nil
	}
	keys := make(chan error, 1)
	safe.Go("cast keys", func() { keys <- r.prepare() })
	if dhcp.Get().Wait(ctx) && r.Enabled() {
		var err error
		select {
		case err = <-keys:
		default:
			r.booted(component.Progress{Doing: "making keys"})
			err = <-keys
		}
		if err == nil {
			err = r.up()
		}
		if err != nil {
			slog.Error("the cast receiver would not start", "err", err)
			r.booted(component.Progress{Failed: true, Doing: err.Error()})
		} else {
			r.booted(component.Progress{Done: true, Doing: fmt.Sprintf("port %d", cast.Port)})
		}
	}
	<-ctx.Done()
	return nil
}

func (r *Receiver) booted(p component.Progress) { r.boot.Store(&p) }

func (r *Receiver) Startup() component.Progress {
	if p := r.boot.Load(); p != nil {
		return *p
	}
	return component.Progress{Doing: "waiting for the network"}
}

// Close takes it down, so a restart does not leave a socket held or a service advertised.
func (r *Receiver) Close() error {
	r.down()
	return nil
}

// SetReceiver turns it on or off. Home Assistant's switch and the harness both come here.
type registrar interface {
	ForgetRegistration()
	Unsave()
	Registered() bool
}

func (r *Receiver) registrars() []registrar {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []registrar
	for _, p := range r.protocols {
		if g, ok := p.(registrar); ok {
			out = append(out, g)
		}
	}
	return out
}

func (r *Receiver) PrimeRegistered() bool {
	for _, g := range r.registrars() {
		if g.Registered() {
			return true
		}
	}
	return false
}

func (r *Receiver) ResetPrime() {
	for _, g := range r.registrars() {
		g.ForgetRegistration()
	}
	slog.Info("prime video registration reset")
}

func (r *Receiver) SetPrimeSkipIntro(on bool) {
	if err := config.Set().Cast().PrimeSkipIntro(on); err != nil {
		slog.Error("saving the prime video skip intro setting failed", "err", err)
	}
}

func (r *Receiver) SetPrimePersist(on bool) {
	if err := config.Set().Cast().PrimePersist(on); err != nil {
		slog.Error("saving the prime video registration setting failed", "err", err)
		return
	}
	if !on {
		for _, g := range r.registrars() {
			g.Unsave()
		}
	}
}

func (r *Receiver) SetReceiver(on bool) {
	r.enable.Set(on)

	if err := config.Set().Cast().Receiver(on); err != nil {
		slog.Error("saving the cast setting failed", "err", err)
		return
	}

	if !on {
		r.down()
		r.booted(component.Progress{Done: true, Doing: "off"})
		return
	}
	if err := r.up(); err != nil {
		slog.Error("the cast receiver would not start", "err", err)
		r.booted(component.Progress{Failed: true, Doing: err.Error()})
		return
	}
	r.booted(component.Progress{Done: true, Doing: fmt.Sprintf("port %d", cast.Port)})
}

// up opens the socket and puts the service on the network.
func deviceName() string {
	if name := config.Get().Device.Name; name != "" {
		return name
	}
	return layout.DefaultName
}

func (r *Receiver) keyed(name string) error {
	if r.keys != nil {
		return nil
	}
	k, err := newKeys(name, layout.CastCredentialsPath)
	if err != nil {
		return err
	}
	k.changed = r.credentials
	r.credentials(k.Current())
	r.keys = k
	return nil
}

func (r *Receiver) prepare() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.keyed(deviceName())
}

func (r *Receiver) up() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.listener != nil {
		return nil
	}

	name := deviceName()
	if err := r.keyed(name); err != nil {
		return err
	}

	listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", cast.Port), cast.TLS(r.keys.Current))
	if err != nil {
		return fmt.Errorf("cast: listening on %d: %w", cast.Port, err)
	}

	device := cast.Identify(hardware(), name, layout.Model)

	advert, err := cast.NewAdvertiser(device, cast.Port)
	if err != nil {
		listener.Close()
		return err
	}

	service := cast.NewService(cast.NewReceiver(name))
	service.Fault = func(err error) { slog.Debug("a cast sender sent something unreadable", "err", err) }

	// What is playing goes back into the record, so the cast menu says what the room is doing
	// rather than only that the device exists.
	unspoken := map[string]bool{}
	service.Receiver.Launched = func(app string) {
		unspoken = map[string]bool{}
		slog.Info("a cast app launched", "app", cast.AppName(app), "id", app)
		advert.Playing(true, cast.AppName(app))
	}
	service.Receiver.Stopped = func(string) { advert.Playing(false, "") }
	service.Receiver.Loaded = func(m cast.Media) {
		slog.Info("a cast sender loaded", "type", m.ContentType, "stream", m.StreamType,
			"content", contentShape(m.ContentID), "custom", len(m.CustomData) > 0)
	}
	service.Receiver.Unspoken = func(namespace, kind string) {
		if unspoken[namespace] {
			return
		}
		unspoken[namespace] = true
		slog.Info("a cast sender used a namespace this does not speak", "namespace", namespace, "type", kind)
	}
	service.Receiver.Volume = r.loud.asked
	service.Receiver.Credentials = r.keys.Current
	service.Receiver.CRL = r.keys.CRL
	service.Receiver.Identity = func() cast.Device { return device }
	if r.protocols == nil {
		r.protocols = cast.Protocols(cast.Env{
			Name:      name,
			Model:     layout.Model,
			Device:    device,
			HTTP:      fetch.Client(30 * time.Second),
			Long:      fetch.Client(0),
			Video:     video.Target,
			Volume:    func() int { return config.Get().Volume.Media },
			SetVolume: func(level int) { volume.Get().Set(config.StreamMedia, level) },
			VolumeChanged: func(do func()) func() {
				return volume.Get().Changed.Listen(func(c volume.Change) {
					if c.Stream == config.StreamMedia {
						do()
					}
				})
			},
			Publish: func(control cast.Playing, p *cast.Published) {
				r.mu.Lock()
				s := r.service
				r.mu.Unlock()
				if s != nil {
					s.Publish(control, p)
				}
			},
			Keep: keep,
			Send: func(m cast.Message) {
				r.mu.Lock()
				s := r.service
				r.mu.Unlock()
				if s != nil {
					s.Send(m)
				}
			},
			Output: r.output(),
			Resample: func(from int) func([]int16) []int16 {
				return speaker.NewRational(from, speaker.Rate, speaker.Channels).Run
			},
			Surface: func() *surface.Client { return display.Get().Helper() },
		})
	}
	for _, p := range r.protocols {
		service.Receiver.Register(p)
	}
	started := time.Now()
	service.Receiver.Eureka = func() cast.Eureka {
		e := cast.NewEureka(device, layout.Model, layout.Version)
		e.IP = config.Get().Network.Address
		e.Timezone = config.Get().Time.Chosen
		e.Uptime = time.Since(started).Seconds()
		return e
	}
	service.Receiver.Setup = func(kind string, data json.RawMessage) {
		slog.Info("a cast sender sent a setup request", "type", kind, "data", string(data))
	}
	service.Receiver.Challenged = func(c cast.Challenge, answered bool) {
		slog.Info("a cast sender asked for device authentication",
			"hash", c.Hash, "algorithm", c.Algorithm, "nonce", len(c.Nonce) > 0, "answered", answered)
	}

	ctx, stop := context.WithCancel(context.Background())
	keys := r.keys
	safe.Go("cast credentials", func() { keys.run(ctx) })
	for _, p := range r.protocols {
		safe.Go("cast protocol "+p.Name(), func() { p.Run(ctx) })
	}

	r.setup = &http.Server{
		Addr:              fmt.Sprintf(":%d", cast.SetupPort),
		ReadHeaderTimeout: 5 * time.Second,
		Handler: cast.SetupHandler(service.Receiver.Eureka, ui.LogoPNG, func(req *http.Request, status int) {
			slog.Info("a cast sender asked the setup endpoint", "peer", req.RemoteAddr, "method", req.Method,
				"url", req.URL.String(), "status", status)
		}),
	}
	setup := r.setup
	safe.Go("cast setup", func() {
		if err := setup.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("the cast setup endpoint stopped", "port", cast.SetupPort, "err", err)
		}
	})

	r.listener, r.advert, r.service, r.stop = listener, advert, service, stop
	r.loud.serve(service)
	safe.Go("cast accept", func() { r.accept(ctx, listener, service) })

	host := layout.Slug(name)
	safe.Go("cast advertise", func() {
		network.Advertise(ctx, "cast", func(ips []net.IP) (func(), error) {
			if err := advert.On(host, network.Strings(ips)); err != nil {
				return nil, err
			}
			return advert.Close, nil
		})
	})

	slog.Info("the cast receiver is up", "name", name, "port", cast.Port)
	return nil
}

// contentShape is a contentId without its path or query, which can carry tokens.
func contentShape(id string) string {
	if u, err := url.Parse(id); err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") {
		return u.Scheme + "://" + u.Host
	}
	return fmt.Sprintf("opaque, %d characters", len(id))
}

// hardware is the wifi address, which is what the device's identity on the network is derived
// from. Empty if it cannot be read: the identity is then derived from nothing, which is stable for
// this device and the same on every device, so a house with two of them would see one. Rare enough
// to log rather than refuse to start over.
func hardware() string {
	raw, err := os.ReadFile(layout.MACPath)
	if err != nil {
		slog.Warn("the cast identity has no hardware address to come from", "err", err)
		return ""
	}
	return layout.MAC(string(raw))
}

// accept serves senders until the listener closes.
func (r *Receiver) accept(ctx context.Context, listener net.Listener, service *cast.Service) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Closing the listener is how this loop is stopped, so that is not a fault.
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			slog.Warn("a cast sender could not be accepted", "err", err)
			continue
		}

		safe.Go("cast sender", func() {
			c := cast.NewConn(conn)
			peer := c.Peer()
			if !DebugLog {
				if err := service.Serve(c); err != nil {
					slog.Debug("a cast sender went away", "err", err)
				}
				return
			}

			c.Trace = func(in bool, m cast.Message) { trace(peer, in, m) }
			slog.Info("cast connection opened", "peer", peer)
			err := service.Serve(c)
			slog.Info("cast connection closed", "peer", peer, "err", err)
		})
	}
}

// DebugLog logs every cast connection and every message on it in full.
const DebugLog = true

// trace logs one message in full, JSON as it is and protobuf as hex.
func trace(peer string, in bool, m cast.Message) {
	if m.Namespace == cast.NSHeartbeat || m.Namespace == cast.NSDeviceAuth {
		return
	}
	dir := "out"
	if in {
		dir = "in"
	}
	body := m.Payload
	if m.Binary != nil {
		body = hex.EncodeToString(m.Binary)
	}
	slog.Info("cast "+dir, "peer", peer, "from", m.Source, "to", m.Destination,
		"namespace", m.Namespace, "body", body)
}

// down closes the socket and takes the service off the network.
func (r *Receiver) down() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.loud.serve(nil)
	if r.stop != nil {
		r.stop()
		r.stop = nil
	}
	if r.advert != nil {
		r.advert.Close()
		r.advert = nil
	}
	if r.listener != nil {
		r.listener.Close()
		r.listener = nil
	}
	if r.setup != nil {
		r.setup.Close()
		r.setup = nil
	}
	if r.service != nil {
		r.service.Receiver.Forget()
		r.service = nil
	}
}

// Playing is what a sender has given the device, for anything that wants to draw it. Nil when
// nothing is.
func (r *Receiver) Playing() *cast.Media {
	r.mu.Lock()
	service := r.service
	r.mu.Unlock()

	if service == nil {
		return nil
	}
	return service.Receiver.Media()
}
