package cast

import (
	"slices"
	"sync"
	"testing"
)

// published records what reached the network, standing in for the registration so the refreshing
// can be checked without putting anything on a network.
type published struct {
	mu    sync.Mutex
	sets  [][]string
	downs int
}

func (p *published) SetText(text []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.sets = append(p.sets, slices.Clone(text))
}

func (p *published) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.downs++
}

func (p *published) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.sets)
}

func (p *published) last() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.sets) == 0 {
		return nil
	}
	return p.sets[len(p.sets)-1]
}

func advertising(t *testing.T) (*Advertiser, *published) {
	t.Helper()

	on := &published{}
	d := Identify("00:f4:8d:47:69:21", "Kitchen", "Lenovo Smart Display 10")

	return Announcing(on, d), on
}

// Every refresh is a multicast announcement to the whole network, and the status changes on every
// track, pause and seek. Announcing each one is a device that talks more than it plays.
func TestNothingIsAnnouncedUnlessTheRecordChanged(t *testing.T) {
	a, on := advertising(t)

	if a.Update(a.Device()) {
		t.Error("an unchanged device was announced")
	}
	if got := on.count(); got != 0 {
		t.Errorf("%d announcements for no change", got)
	}

	// The same status set twice is one announcement.
	if !a.Playing(true, "Sun Ra") {
		t.Error("a change was not announced")
	}
	if a.Playing(true, "Sun Ra") {
		t.Error("the same status was announced twice")
	}
	if got := on.count(); got != 1 {
		t.Errorf("%d announcements, want one", got)
	}
}

// st and rs are what a sender shows as the device being busy and what it is doing.
func TestWhatPlayingPutsInTheRecord(t *testing.T) {
	a, on := advertising(t)

	a.Playing(true, "Sun Ra — Space Is The Place")

	got := on.last()
	if !slices.Contains(got, "st=1") {
		t.Errorf("the record does not say the device is busy: %q", got)
	}
	if !slices.Contains(got, "rs=Sun Ra — Space Is The Place") {
		t.Errorf("the record does not carry the status: %q", got)
	}

	a.Playing(false, "")

	got = on.last()
	if !slices.Contains(got, "st=0") {
		t.Errorf("the record still says busy: %q", got)
	}
	if !slices.Contains(got, "rs=") {
		t.Errorf("the status was not cleared: %q", got)
	}
}

// The identity does not move when the status does. A sender remembers the id, and one that changed
// would be a new device in the cast menu every track.
func TestTheIdentityDoesNotMoveWithTheStatus(t *testing.T) {
	a, on := advertising(t)
	was := a.Device().ID

	a.Playing(true, "something")

	if now := a.Device().ID; now != was {
		t.Errorf("the id changed from %q to %q", was, now)
	}
	if !slices.Contains(on.last(), "id="+was) {
		t.Errorf("the announced id is not the device's: %q", on.last())
	}
}

// Renaming the device is a change like any other, and has to reach the network.
func TestRenamingIsAnnounced(t *testing.T) {
	a, on := advertising(t)

	d := a.Device()
	d.Name = "Front Room"

	if !a.Update(d) {
		t.Fatal("a rename was not announced")
	}
	if !slices.Contains(on.last(), "fn=Front Room") {
		t.Errorf("the new name did not go out: %q", on.last())
	}
}

// An unannounced departure leaves the record in every resolver's cache until it expires, and a
// device in the cast menu that does not answer is worse than one that is not there.
func TestClosingTakesTheServiceOff(t *testing.T) {
	a, on := advertising(t)

	a.Close()
	if on.downs != 1 {
		t.Errorf("shut down %d times", on.downs)
	}

	// And twice is not twice: a component stopped and then stopped again must not panic on a
	// registration that is already gone.
	a.Close()
	if on.downs != 1 {
		t.Errorf("shut down %d times after closing twice", on.downs)
	}
}

// What is playing changes on the audio path and the name changes in the settings, and neither
// knows about the other.
func TestTheAdvertiserTakesChangesFromEverywhereAtOnce(t *testing.T) {
	a, _ := advertising(t)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := range 16 {
				if i%2 == 0 {
					a.Playing(j%2 == 0, "track "+itoa(j))
					continue
				}

				d := a.Device()
				d.Name = "Room " + itoa(j)
				a.Update(d)
			}
		}()
	}
	wg.Wait()

	// Nothing to assert about the final state — the point is that it is reached without a race and
	// without a panic, which is what -race and this test together say.
	if a.Device().ID == "" {
		t.Error("the device lost its identity")
	}
}

// A record a sender quietly ignores is worse than an error: the device is on the network and not
// in the menu, and nothing says why.
func TestAdvertisingRefusesADeviceThatWouldNotBeOffered(t *testing.T) {
	if _, err := NewAdvertiser(Device{Name: "Kitchen"}, Port); err == nil {
		t.Error("a device with no id was advertised")
	}
	if _, err := NewAdvertiser(Device{ID: "bad"}, Port); err == nil {
		t.Error("a device with a short id was advertised")
	}
}
