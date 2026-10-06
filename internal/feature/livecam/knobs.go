package livecam

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/setting"
)

type Knobs struct {
	EV                                               int
	Scene, Banding, ISO, WhiteBalance, Effect        string
	Brightness, Contrast, Saturation, Sharpness, Hue string
	Noise                                            string
	RateAuto                                         bool
	Rate                                             int
	MainOn, SubOn                                    bool
	MainSize                                         string
	SubWidth, Keyframe                               int
	Quality                                          string
}

type Knob = setting.Setting[Knobs]

const (
	PictureGroup  setting.Group = "Picture"
	ExposureGroup setting.Group = "Exposure"
	BalanceGroup  setting.Group = "White Balance"
)

type vendor struct {
	evMin, evMax int
	scenes       []string
	balances     []string
	effects      []string
	isos         []string
	levels       bool
	sliders      bool
	noises       []string
	rates        []int
}

var qualcomm = vendor{
	evMin: -12, evMax: 12,
	scenes: []string{"auto", "landscape", "snow", "beach", "sunset", "night", "portrait", "sports", "steadyphoto",
		"candlelight", "fireworks", "party", "night-portrait", "theatre", "action", "hdr"},
	balances: []string{"auto", "incandescent", "fluorescent", "warm-fluorescent", "daylight", "cloudy-daylight",
		"twilight", "shade"},
	effects: []string{"none", "mono", "negative", "solarize", "sepia", "posterize", "whiteboard", "blackboard", "aqua"},
	sliders: true,
	noises:  []string{"off", "fast", "high_quality", "minimal"},
	rates:   []int{15, 20, 24, 30},
}

const (
	sliderMax    = 10
	sliderMiddle = "5"
	sharpSteps   = 6
	sharpStep    = 6
	sharpDefault = "2"
)

var mediatek = vendor{
	evMin: -1, evMax: 1,
	scenes: []string{"auto", "portrait", "landscape", "night", "night-portrait", "theatre", "beach", "snow", "sunset",
		"steadyphoto", "sports", "party", "candlelight"},
	balances: []string{"auto", "incandescent", "fluorescent", "daylight", "cloudy-daylight", "twilight", "shade"},
	effects:  []string{"none", "mono", "negative", "sepia", "aqua", "whiteboard", "blackboard"},
	isos:     []string{"auto", "100", "200", "400", "800", "1600"},
	levels:   true,
	rates:    []int{15, 20, 30},
}

var bandings = []string{"auto", "off", "50hz", "60hz"}
var levels = []string{"low", "middle", "high"}

func hal() vendor {
	if board.Current().SoC == board.MediaTek {
		return mediatek
	}
	return qualcomm
}

func DefaultKnobs() Knobs {
	k := Knobs{Scene: "auto", Banding: "auto", ISO: "auto", WhiteBalance: "auto", Effect: "none",
		Brightness: "middle", Contrast: "middle", Saturation: "middle", Sharpness: "middle", Hue: "middle",
		RateAuto: true, Rate: FPS,
		MainOn: true, SubOn: board.Current().SubWidth > 0, MainSize: defaultMainSize(),
		SubWidth: max(board.Current().SubWidth, subWidths[len(subWidths)-1]), Keyframe: Keyframe, Quality: "standard"}
	if hal().sliders {
		k.Brightness, k.Contrast, k.Saturation, k.Sharpness = sliderMiddle, sliderMiddle, sliderMiddle, sharpDefault
		k.Noise = "fast"
	}
	return k
}

func nearest(rates []int, n int) int {
	best := rates[0]
	for _, r := range rates {
		if abs(r-n) < abs(best-n) {
			best = r
		}
	}
	return best
}

func abs(n int) int { return max(n, -n) }

func rateRows(rates []int) []Knob {
	auto := Knob{Name: "framerate_auto", Kind: setting.Toggle, Group: ExposureGroup, Icon: "mdi:speedometer",
		Read: func(k *Knobs) string { return setting.OnOff(k.RateAuto) }}
	auto.Write = func(k *Knobs, v string) error {
		on, ok := setting.Boolean(v)
		if !ok {
			return auto.Bad(v, "on or off")
		}
		k.RateAuto = on
		return nil
	}
	fixed := Knob{Name: "framerate", Kind: setting.Number, Group: ExposureGroup, Icon: "mdi:filmstrip",
		Slider: true, Min: rates[0], Max: rates[len(rates)-1], Unit: "fps", IdleAs: "Auto",
		Idle: func(k *Knobs) bool { return k.RateAuto },
		Read: func(k *Knobs) string { return strconv.Itoa(k.Rate) }}
	fixed.Write = func(k *Knobs, s string) error {
		n, err := strconv.Atoi(s)
		if err != nil {
			return fixed.Bad(s, fmt.Sprintf("between %d and %d", rates[0], rates[len(rates)-1]))
		}
		k.Rate = nearest(rates, n)
		return nil
	}
	return []Knob{auto, fixed}
}

func number(name string, g setting.Group, icon string, max int, field func(*Knobs) *string) Knob {
	k := Knob{Name: name, Kind: setting.Number, Group: g, Icon: icon, Slider: true, Min: 0, Max: max,
		Read: func(k *Knobs) string { return *field(k) }}
	k.Write = func(at *Knobs, s string) error {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || n > max {
			return k.Bad(s, fmt.Sprintf("between 0 and %d", max))
		}
		*field(at) = strconv.Itoa(n)
		return nil
	}
	return k
}

func words(values []string) []setting.Option {
	out := make([]setting.Option, len(values))
	for i, v := range values {
		out[i] = setting.Option{Value: v, Key: v}
	}
	return out
}

func labels(values []string) []setting.Option {
	out := make([]setting.Option, len(values))
	for i, v := range values {
		out[i] = setting.Option{Value: v, Label: v}
	}
	return out
}

func choice(name string, g setting.Group, icon string, opts []setting.Option, field func(*Knobs) *string) Knob {
	k := Knob{Name: name, Kind: setting.Choice, Group: g, Icon: icon, Options: opts,
		Read: func(k *Knobs) string { return *field(k) }}
	k.Write = func(at *Knobs, v string) error {
		for _, o := range opts {
			if o.Value == v {
				*field(at) = v
				return nil
			}
		}
		return k.Bad(v, "not one of its options")
	}
	return k
}

func rows() []Knob {
	v := hal()
	ev := Knob{Name: "ev", Kind: setting.Number, Group: ExposureGroup, Icon: "mdi:plus-minus-variant", Slider: true,
		Min: v.evMin, Max: v.evMax,
		Read: func(k *Knobs) string { return strconv.Itoa(k.EV) }}
	ev.Write = func(k *Knobs, s string) error {
		n, err := strconv.Atoi(s)
		if err != nil || n < v.evMin || n > v.evMax {
			return ev.Bad(s, fmt.Sprintf("between %d and %d", v.evMin, v.evMax))
		}
		k.EV = n
		return nil
	}

	out := []Knob{
		ev,
		choice("scene", ExposureGroup, "mdi:image-filter-hdr", words(v.scenes), func(k *Knobs) *string { return &k.Scene }),
		choice("banding", ExposureGroup, "mdi:sine-wave", words(bandings), func(k *Knobs) *string { return &k.Banding }),
	}
	if len(v.isos) > 0 {
		out = append(out, choice("iso", ExposureGroup, "mdi:film", labels(v.isos), func(k *Knobs) *string { return &k.ISO }))
	}
	out = append(out,
		choice("whitebalance", BalanceGroup, "mdi:white-balance-auto", words(v.balances),
			func(k *Knobs) *string { return &k.WhiteBalance }),
		choice("effect", PictureGroup, "mdi:image-filter-black-white", words(v.effects),
			func(k *Knobs) *string { return &k.Effect }),
	)
	if v.levels {
		out = append(out,
			choice("brightness", PictureGroup, "mdi:brightness-6", words(levels), func(k *Knobs) *string { return &k.Brightness }),
			choice("contrast", PictureGroup, "mdi:contrast-box", words(levels), func(k *Knobs) *string { return &k.Contrast }),
			choice("saturation", PictureGroup, "mdi:palette", words(levels), func(k *Knobs) *string { return &k.Saturation }),
			choice("sharpness", PictureGroup, "mdi:blur-off", words(levels), func(k *Knobs) *string { return &k.Sharpness }),
			choice("hue", PictureGroup, "mdi:looks", words(levels), func(k *Knobs) *string { return &k.Hue }),
		)
	}
	out = append(out, rateRows(v.rates)...)
	out = append(out, streamRows()...)
	if v.sliders {
		out = append(out,
			number("brightness", PictureGroup, "mdi:brightness-6", sliderMax, func(k *Knobs) *string { return &k.Brightness }),
			number("contrast", PictureGroup, "mdi:contrast-box", sliderMax, func(k *Knobs) *string { return &k.Contrast }),
			number("saturation", PictureGroup, "mdi:palette", sliderMax, func(k *Knobs) *string { return &k.Saturation }),
			number("sharpness", PictureGroup, "mdi:blur-off", sharpSteps, func(k *Knobs) *string { return &k.Sharpness }),
			choice("noise", PictureGroup, "mdi:grain", words(v.noises), func(k *Knobs) *string { return &k.Noise }),
		)
	}
	return out
}

var (
	table *setting.Table[Knobs]
	built sync.Once
)

func Table() *setting.Table[Knobs] {
	built.Do(func() {
		table = setting.NewTable("camera", []setting.Group{PictureGroup, ExposureGroup, BalanceGroup, StreamGroup}, rows())
	})
	return table
}

// Saved is the defaults with the saved knobs over them.
func Saved() Knobs {
	k, bad := Table().Configured(DefaultKnobs(), config.Get().Camera.Settings)
	for _, err := range bad {
		slog.Warn("a saved camera setting no longer applies", "err", err)
	}
	return k
}

// Params is k as the vendor camera's parameters.
func Params(k Knobs) string {
	p := []string{
		"exposure-compensation=" + strconv.Itoa(k.EV),
		"scene-mode=" + k.Scene,
		"antibanding=" + k.Banding,
		"whitebalance=" + k.WhiteBalance,
		"effect=" + k.Effect,
	}
	if hal().levels {
		p = append(p, "iso-speed="+k.ISO, "brightness="+k.Brightness, "contrast="+k.Contrast,
			"saturation="+k.Saturation, "edge="+k.Sharpness, "hue="+k.Hue)
	}
	if k.RateAuto {
		p = append(p, "fps-range=auto")
	} else {
		r := nearest(hal().rates, k.Rate)
		p = append(p, fmt.Sprintf("fps-range=%d,%d", r, r))
	}
	if v := hal(); v.sliders {
		sharp, _ := strconv.Atoi(k.Sharpness)
		p = append(p, "brightness="+k.Brightness, "contrast="+k.Contrast, "saturation="+k.Saturation,
			"sharpness="+strconv.Itoa(sharp*sharpStep),
			"noise-reduction="+strconv.Itoa(max(0, slices.Index(v.noises, k.Noise))))
	}
	return strings.Join(p, ";")
}

// Set saves one knob and applies it to a running camera.
func Set(name, value string) error {
	if err := config.Set().Camera().Set(name, value); err != nil {
		return err
	}
	Apply()
	if name == "main_on" || name == "sub_on" {
		StreamsChanged.Emit(Served())
	}
	return nil
}

// Reset puts every knob back to its default.
func Reset() error {
	if err := config.Set().Camera().Clear(); err != nil {
		return err
	}
	Apply()
	controls().Publish()
	StreamsChanged.Emit(Served())
	return nil
}

// Apply sends the saved knobs to the camera, if it is running.
func Apply() {
	if err := hub.SetParams(Params(Saved())); err != nil {
		slog.Warn("the camera settings could not be applied", "err", err)
	}
}

func init() {
	component.Register(component.Device, Get, component.Order(36))
}

type Camera struct{}

var cam = &Camera{}

func Get() *Camera { return cam }

func (*Camera) Name() string { return "camera" }

var (
	knobs     *setting.Controls[Knobs]
	knobsOnce sync.Once
)

func controls() *setting.Controls[Knobs] {
	knobsOnce.Do(func() {
		knobs = &setting.Controls[Knobs]{
			Table:  Table(),
			Device: component.DeviceCamera,
			Want:   func(setting.Group) bool { return true },
			Read:   Saved,
			Save:   func(s Knob, v string) error { return Set(s.Name, v) },
		}
	})
	return knobs
}

func (*Camera) Entities() []esphome.Entity {
	reset := &esphome.Button{
		Base: esphome.Base{ObjectID: "camera_reset", Name: "Reset camera settings", Icon: "mdi:restore",
			Category: esphome.CategoryConfig, DeviceID: component.DeviceCamera},
		OnPress: func() {
			if err := Reset(); err != nil {
				slog.Error("resetting the camera settings", "err", err)
			}
		},
	}
	return append(controls().Entities(), reset)
}

// Restore publishes what the knobs are set to, so the entities do not come up empty.
func (*Camera) Restore(config.Config) { controls().Publish() }
