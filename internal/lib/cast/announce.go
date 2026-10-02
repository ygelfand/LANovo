package cast

import (
	"fmt"
	"slices"
	"sync"

	"github.com/libp2p/zeroconf/v2"
)

// Putting the service on the network, and keeping what it says true.
//
// The records are not static. st says whether something is running and rs is the line under the
// name, so both change every time a track does — and a sender that looked before the change is
// still showing what it saw. Refreshing is what makes the cast menu agree with the room.
//
// It is also the thing to be careful with. Every refresh is a multicast announcement to the whole
// network, and the status changes on every track, every pause and every seek. Announcing each one
// is a device that talks more than it plays, so nothing goes out unless the record actually
// changed.

// Publisher is what holds the registration. zeroconf's Server is one; a test provides its own.
type Publisher interface {
	SetText(text []string)
	Shutdown()
}

// Advertiser is the service as it currently stands.
//
// Safe for concurrent use: what is playing changes on the audio path and the name changes in the
// settings, and neither knows about the other.
type Advertiser struct {
	mu sync.Mutex

	on     Publisher
	device Device
	last   []string
	port   int
}

// NewAdvertiser is the service ready to go on the network, which On puts it on.
func NewAdvertiser(d Device, port int) (*Advertiser, error) {
	if err := d.Valid(); err != nil {
		return nil, err
	}
	return &Advertiser{device: d, last: d.Records(), port: port}, nil
}

// On publishes the service as host at these addresses, replacing whatever it was published as.
func (a *Advertiser) On(host string, ips []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.on != nil {
		a.on.Shutdown()
		a.on = nil
	}

	// zeroconf wants the trailing dot on the domain.
	server, err := zeroconf.RegisterProxy(a.device.Instance(), ServiceType, "local.", a.port, host, ips, a.last, nil)
	if err != nil {
		return fmt.Errorf("cast: advertising %q: %w", a.device.Name, err)
	}
	a.on = server
	return nil
}

// Announcing is an advertiser over a publisher that is already registered, which is what lets the
// refreshing be tested without putting anything on a network.
func Announcing(on Publisher, d Device) *Advertiser {
	return &Advertiser{on: on, device: d, last: d.Records()}
}

// Update changes what the service says, and announces only if that is different.
//
// Reports whether anything went out, which is worth knowing when the caller is a status that ticks.
func (a *Advertiser) Update(d Device) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	records := d.Records()
	if slices.Equal(records, a.last) {
		return false
	}

	a.device = d
	a.last = records
	if a.on != nil {
		a.on.SetText(records)
	}

	return true
}

// Playing is the common change: something started or stopped, and what it is.
func (a *Advertiser) Playing(running bool, status string) bool {
	a.mu.Lock()
	d := a.device
	a.mu.Unlock()

	d.Running = running
	d.Status = status

	return a.Update(d)
}

// Device is what is currently advertised.
func (a *Advertiser) Device() Device {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.device
}

// Close takes the service off the network.
//
// Worth doing rather than letting the process exit: an unannounced departure leaves the record in
// every resolver's cache until it expires, and a device in the cast menu that does not answer is
// worse than one that is not there.
func (a *Advertiser) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.on != nil {
		a.on.Shutdown()
		a.on = nil
	}
}
