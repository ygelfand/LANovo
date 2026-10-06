// Package visual adapts shared visual renderers to product labels and artwork.
package visual

import (
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/libcountertop/pkg/display/ui"
	shared "github.com/ygelfand/libcountertop/pkg/display/visual"
	"slices"
	"strings"
)

type Kind shared.Kind

const LensFlares Kind = Kind(shared.LensFlares)
const ClassicVU Kind = Kind(shared.ClassicVU)
const Aurora Kind = Kind(shared.Aurora)
const Mercury Kind = Kind(shared.Mercury)
const Ribbons Kind = Kind(shared.Ribbons)
const Galaxy Kind = Kind(shared.Galaxy)
const Halo Kind = Kind(shared.Halo)
const Pulsar Kind = Kind(shared.Pulsar)
const Phosphor Kind = Kind(shared.Phosphor)
const Tesla Kind = Kind(shared.Tesla)
const Orb Kind = Kind(shared.Orb)
const DigitalVU Kind = Kind(shared.DigitalVU)
const PaintSplash Kind = Kind(shared.PaintSplash)
const Fireworks Kind = Kind(shared.Fireworks)
const Synthwave Kind = Kind(shared.Synthwave)
const HAL9000 Kind = Kind(shared.HAL9000)
const Mother Kind = Kind(shared.Mother)
const LightCycles Kind = Kind(shared.LightCycles)
const LCARS Kind = Kind(shared.LCARS)
const PipBoy3 Kind = Kind(shared.PipBoy3)
const PipBoyNV Kind = Kind(shared.PipBoyNV)
const PipBoy4 Kind = Kind(shared.PipBoy4)
const PipBoy76 Kind = Kind(shared.PipBoy76)
const KoiPond Kind = Kind(shared.KoiPond)
const RainOnGlass Kind = Kind(shared.RainOnGlass)
const Cymatics Kind = Kind(shared.Cymatics)
const Fire Kind = Kind(shared.Fire)
const NeonSign Kind = Kind(shared.NeonSign)
const LavaLamp Kind = Kind(shared.LavaLamp)
const InkInWater Kind = Kind(shared.InkInWater)
const Thunderstorm Kind = Kind(shared.Thunderstorm)
const Fireflies Kind = Kind(shared.Fireflies)
const Matrix Kind = Kind(shared.Matrix)
const Default = ClassicVU

type Traits = shared.Traits
type Input = shared.Input
type Visual = shared.Visual
type Passes = shared.Passes
type GL = shared.GL
type Quad = shared.Quad
type Segment = shared.Segment
type Point = shared.Point
type Layout = shared.Layout

const Light = shared.Light
const Pre = shared.Pre
const Feed = shared.Feed
const FeedHalf = shared.FeedHalf
const Splat = shared.Splat
const SplatHalf = shared.SplatHalf
const Lines = shared.Lines
const FeedFloat = shared.FeedFloat
const Full = shared.Full
const Compact = shared.Compact
const SeedMost = shared.SeedMost

var LayoutOf = shared.LayoutOf
var Portrait = shared.Portrait
var SetSeed = shared.SetSeed

func (k Kind) Traits() Traits { return shared.Kind(k).Traits() }
func (k Kind) Label() string {
	if !slices.Contains(shared.Kinds(), shared.Kind(k)) {
		k = Default
	}
	return say.T("visual." + string(k))
}
func kinds(in []shared.Kind) []Kind {
	out := make([]Kind, len(in))
	for i, k := range in {
		out[i] = Kind(k)
	}
	return out
}
func Kinds() []Kind { return kinds(shared.Kinds()) }
func Built() []Kind {
	out := kinds(shared.Built())
	slices.SortFunc(out, func(a, b Kind) int { return strings.Compare(strings.ToLower(a.Label()), strings.ToLower(b.Label())) })
	return out
}

type branded struct{ Visual }

func (v branded) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if strings.TrimSpace(x.Label) == "" {
		x.Label = "LANOVO"
	}
	return v.Visual.Shade(g, fresh, in, x)
}
func New(k Kind) Visual { return branded{Visual: shared.New(shared.Kind(k))} }
