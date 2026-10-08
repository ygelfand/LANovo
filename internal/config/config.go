package config

type Config struct {
	Device Device `json:"-"`
	Screen Screen `json:"screen"`
	Clock  Clock  `json:"clock"`
	Idle   Idle   `json:"idle"`
	Volume Volume `json:"volume"`
	Media  Media  `json:"media"`
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

func Defaults() Config {
	return Config{
		Screen: defaultScreen(),
		Clock:  defaultClock(),
		Idle:   defaultIdle(),
		Volume: defaultVolume(),
		Media:  defaultMedia(),
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

type Device struct {
	Name  string
	Addr  string
	Model string
}

type Writer struct{ st *Store }

func (w Writer) Screen() ScreenWriter         { return ScreenWriter(w) }
func (w Writer) Clock() ClockWriter           { return ClockWriter(w) }
func (w Writer) Idle() IdleWriter             { return IdleWriter(w) }
func (w Writer) Media() MediaWriter           { return MediaWriter(w) }
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

type Labeled interface{ Label() string }

func Labels[T Labeled](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.Label())
	}
	return out
}

func ByLabel[T Labeled](values []T, label string) (T, bool) {
	for _, v := range values {
		if v.Label() == label {
			return v, true
		}
	}
	var zero T
	return zero, false
}
