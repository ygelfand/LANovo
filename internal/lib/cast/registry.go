package cast

import (
	"net/http"
	"sort"
	"sync"

	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

// Env is what the device lends every protocol.
type Env struct {
	Name   string
	Model  string
	Device Device

	// HTTP is for ordinary requests; Long must not time out, for protocols that hold a long poll.
	HTTP *http.Client
	Long *http.Client

	Tallest int

	// Volume is the media volume, 0 to 100, and SetVolume moves it.
	Volume    func() int
	SetVolume func(level int)

	// Output plays what a protocol hands it.
	Output playback.Output

	Resample func(from int) func(stereo []int16) []int16

	Surface func() *surface.Client
}

var (
	definedMu sync.Mutex
	defined   = map[string]func(Env) Protocol{}
)

// Define makes a protocol available by name. Protocols call it from init.
func Define(name string, make func(Env) Protocol) {
	definedMu.Lock()
	defer definedMu.Unlock()
	defined[name] = make
}

// Protocols builds every defined protocol, in name order.
func Protocols(env Env) []Protocol {
	definedMu.Lock()
	defer definedMu.Unlock()

	names := make([]string, 0, len(defined))
	for n := range defined {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]Protocol, 0, len(names))
	for _, n := range names {
		out = append(out, defined[n](env))
	}
	return out
}
