// Package config is everything the device is set to, and the one place that decides it.
//
// Reading takes no lock and carries no default:
//
//	c := config.Get()
//	c.Screen.Backlight
//
// Writing names the thing being changed, and persists it:
//
//	config.Set().Screen().Backlight(70)
//
// Each part of the device gets a file here holding its own struct, its defaults and its writer, so
// a setting and everything about it is in one place. Nothing distinguishes "never set" from "set to
// the default": loading starts from the defaults and lets the file write over what it mentions.
//
// The values the user can choose between are named here too, as closed sets of identifiers with a
// label — the identifier is written to the file and branched on, the label is what Home Assistant
// shows. Keeping both here means persistence does not import the subsystem a setting belongs to,
// and the subsystem does not import persistence. It also means a setting has one home whatever
// edits it, so a second way in changes nothing about where the value lives.
package config

// Config is the whole of what the device is set to.
type Config struct {
	Device Device `json:"-"`
	Screen Screen `json:"screen"`
	Clock  Clock  `json:"clock"`
	Idle   Idle   `json:"idle"`
	Volume Volume `json:"volume"`
	Wake   Wake   `json:"wake"`

	Feedback   Feedback   `json:"feedback"`
	Microphone Microphone `json:"microphone"`
	Sendspin   Sendspin   `json:"sendspin"`
	Call       Call       `json:"call"`
	Diag       Diag       `json:"diag"`
	Time       Time       `json:"time"`
	Network    Network    `json:"network"`
	Bluetooth  Bluetooth  `json:"bluetooth"`
	Cast       Cast       `json:"cast"`
	API        API        `json:"api"`
	Access     Access     `json:"access"`
	Camera     Camera     `json:"camera"`
	Visual     Visual     `json:"visual"`
	RTSP       RTSP       `json:"rtsp"`
	Presence   Presence   `json:"presence"`
	Poster     Poster     `json:"poster"`
	Update     Update     `json:"update"`
	Home       Home       `json:"home"`
	Weather    Weather    `json:"weather"`
}

// Defaults is a device nobody has set anything on.
func Defaults() Config {
	return Config{
		Screen: defaultScreen(),
		Clock:  defaultClock(),
		Idle:   defaultIdle(),
		Volume: defaultVolume(),
		Wake:   defaultWake(),

		Feedback:   defaultFeedback(),
		Microphone: defaultMicrophone(),
		Sendspin:   defaultSendspin(),
		Call:       defaultCall(),
		Diag:       defaultDiag(),
		Time:       defaultTime(),
		Network:    defaultNetwork(),
		Bluetooth:  defaultBluetooth(),
		Cast:       defaultCast(),
		API:        defaultAPI(),
		Access:     defaultAccess(),
		Camera:     defaultCamera(),
		Visual:     defaultVisual(),
		RTSP:       defaultRTSP(),
		Presence:   defaultPresence(),
		Poster:     defaultPoster(),
		Update:     defaultUpdate(),
		Home:       defaultHome(),
		Weather:    defaultWeather(),
	}
}

// Device is what lanovod was told at start-up rather than what anyone chose. It is read like
// everything else, and not written to the file: the next process is told again.
//
// Nothing here is readable until boot has called Started, so a component built during init must not
// reach for it.
type Device struct {
	Name  string
	Addr  string
	Model string
}

// Writer is what Set hands back: one method per part of the device, each with its own settings.
//
// Nothing here holds a lock. The leaf call does the whole thing — take the lock, change the value,
// write the file.
type Writer struct{ st *Store }

func (w Writer) Screen() ScreenWriter         { return ScreenWriter(w) }
func (w Writer) Clock() ClockWriter           { return ClockWriter(w) }
func (w Writer) Idle() IdleWriter             { return IdleWriter(w) }
func (w Writer) Volume() VolumeWriter         { return VolumeWriter(w) }
func (w Writer) Wake(slot int) WakeWriter     { return WakeWriter{st: w.st, slot: slot} }
func (w Writer) Stop() StopWriter             { return StopWriter(w) }
func (w Writer) Feedback() FeedbackWriter     { return FeedbackWriter(w) }
func (w Writer) Microphone() MicrophoneWriter { return MicrophoneWriter(w) }
func (w Writer) Sendspin() SendspinWriter     { return SendspinWriter(w) }
func (w Writer) Call() CallWriter             { return CallWriter(w) }
func (w Writer) Diag() DiagWriter             { return DiagWriter(w) }
func (w Writer) API() APIWriter               { return APIWriter(w) }
func (w Writer) Time() TimeWriter             { return TimeWriter(w) }
func (w Writer) Network() NetworkWriter       { return NetworkWriter(w) }
func (w Writer) Bluetooth() BluetoothWriter   { return BluetoothWriter(w) }
func (w Writer) Cast() CastWriter             { return CastWriter(w) }
func (w Writer) Access() AccessWriter         { return AccessWriter(w) }
func (w Writer) Camera() CameraWriter         { return CameraWriter(w) }
func (w Writer) Visual() VisualWriter         { return VisualWriter(w) }
func (w Writer) RTSP() RTSPWriter             { return RTSPWriter(w) }
func (w Writer) Presence() PresenceWriter     { return PresenceWriter(w) }
func (w Writer) Poster() PosterWriter         { return PosterWriter(w) }
func (w Writer) Update() UpdateWriter         { return UpdateWriter(w) }
func (w Writer) Home() HomeWriter             { return HomeWriter(w) }
func (w Writer) Weather() WeatherWriter       { return WeatherWriter(w) }

// Labeled is a setting whose values name themselves. The entity layer binds any of these to a
// select without knowing which setting it is.
type Labeled interface{ Label() string }

// Labels is what Home Assistant shows for a set of values.
func Labels[T Labeled](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.Label())
	}
	return out
}

// ByLabel resolves what Home Assistant sent back to the value it names. A select speaks labels,
// everything else speaks values, and this is the one place the two meet.
func ByLabel[T Labeled](values []T, label string) (T, bool) {
	for _, v := range values {
		if v.Label() == label {
			return v, true
		}
	}
	var zero T
	return zero, false
}
