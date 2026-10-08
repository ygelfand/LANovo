package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"
)

type Config struct {
	Device Device        `json:"-"`
	Screen schema.Screen `json:"screen"`
	Clock  schema.Clock  `json:"clock"`
	Idle   schema.Idle   `json:"idle"`
	Volume Volume        `json:"volume"`
	Media  schema.Media  `json:"media"`
	Wake   schema.Wake   `json:"wake"`

	Feedback   schema.Feedback  `json:"feedback"`
	Microphone Microphone       `json:"microphone"`
	Sendspin   schema.Sendspin  `json:"sendspin"`
	Call       schema.Call      `json:"call"`
	Diag       Diag             `json:"diag"`
	Time       schema.Time      `json:"time"`
	Network    schema.Network   `json:"network"`
	Bluetooth  schema.Bluetooth `json:"bluetooth"`
	Cast       schema.Cast      `json:"cast"`
	API        schema.API       `json:"api"`
	Access     Access           `json:"access"`
	Camera     schema.Camera    `json:"camera"`
	Visual     schema.Visual    `json:"visual"`
	RTSP       schema.RTSP      `json:"rtsp"`
	Presence   Presence         `json:"presence"`
	Poster     schema.Poster    `json:"poster"`
	Update     schema.Update    `json:"update"`
	Home       schema.Home      `json:"home"`
	Weather    schema.Weather   `json:"weather"`
}

func Defaults() Config {
	return Config{
		Screen: schema.DefaultScreen(),
		Clock:  schema.DefaultClock(),
		Idle:   schema.DefaultIdle(),
		Volume: defaultVolume(),
		Media:  schema.DefaultMedia(),
		Wake:   schema.DefaultWake(),

		Feedback:   schema.DefaultFeedback(),
		Microphone: defaultMicrophone(),
		Sendspin:   schema.DefaultSendspin(),
		Call:       schema.DefaultCall(),
		Diag:       defaultDiag(),
		Time:       schema.Time{},
		Network:    schema.DefaultNetwork(),
		Bluetooth:  schema.DefaultBluetooth(),
		Cast:       schema.DefaultCast(),
		API:        schema.DefaultAPI(),
		Access:     defaultAccess(),
		Camera:     schema.DefaultCamera(),
		Visual:     schema.DefaultVisualSettings(),
		RTSP:       schema.DefaultRTSP(),
		Presence:   defaultPresence(),
		Poster:     schema.DefaultPoster(),
		Update:     schema.DefaultUpdate(),
		Home:       schema.DefaultHome(),
		Weather:    schema.DefaultWeather(),
	}
}

type Device struct {
	Name  string
	Addr  string
	Model string
}

type Writer struct{ st *Store }

func (w Writer) Volume() VolumeWriter         { return VolumeWriter(w) }
func (w Writer) Microphone() MicrophoneWriter { return MicrophoneWriter(w) }
func (w Writer) Diag() DiagWriter             { return DiagWriter(w) }
func (w Writer) Access() AccessWriter         { return AccessWriter(w) }
func (w Writer) Presence() PresenceWriter     { return PresenceWriter(w) }

func (w Writer) Screen() schema.ScreenWriter {
	return schema.NewScreenWriter(storage.Field(w.st.Store, screenOf))
}

func (w Writer) Clock() schema.ClockWriter {
	return schema.NewClockWriter(storage.Field(w.st.Store, clockOf))
}

func (w Writer) Idle() schema.IdleWriter {
	return schema.NewIdleWriter(storage.Field(w.st.Store, idleOf))
}

func (w Writer) API() schema.APIWriter {
	return schema.NewAPIWriter(storage.Field(w.st.Store, apiOf))
}

func (w Writer) Media() schema.MediaWriter {
	return schema.NewMediaWriter(storage.Field(w.st.Store, mediaOf))
}

func (w Writer) Wake(slot int) schema.WakeWriter {
	return schema.NewWakeWriter(storage.Field(w.st.Store, wakeOf), slot)
}

func (w Writer) Stop() schema.StopWriter {
	return schema.NewStopWriter(storage.Field(w.st.Store, wakeOf))
}

func (w Writer) Feedback() schema.FeedbackWriter {
	return schema.NewFeedbackWriter(storage.Field(w.st.Store, feedbackOf))
}

func (w Writer) Sendspin() schema.SendspinWriter {
	return schema.NewSendspinWriter(storage.Field(w.st.Store, sendspinOf))
}

func (w Writer) Call() schema.CallWriter {
	return schema.NewCallWriter(storage.Field(w.st.Store, callOf))
}

func (w Writer) Time() schema.TimeWriter {
	return schema.NewTimeWriter(storage.Field(w.st.Store, timeOf))
}

func (w Writer) Network() schema.NetworkWriter {
	return schema.NewNetworkWriter(storage.Field(w.st.Store, networkOf))
}

func (w Writer) Bluetooth() schema.BluetoothWriter {
	return schema.NewBluetoothWriter(storage.Field(w.st.Store, bluetoothOf))
}

func (w Writer) Cast() schema.CastWriter {
	return schema.NewCastWriter(storage.Field(w.st.Store, castOf))
}

func (w Writer) Camera() schema.CameraWriter {
	return schema.NewCameraWriter(storage.Field(w.st.Store, cameraOf))
}

func (w Writer) Visual() schema.VisualWriter {
	return schema.NewVisualWriter(storage.Field(w.st.Store, visualOf))
}

func (w Writer) RTSP() schema.RTSPWriter {
	return schema.NewRTSPWriter(storage.Field(w.st.Store, rtspOf))
}

func (w Writer) Poster() schema.PosterWriter {
	return schema.NewPosterWriter(storage.Field(w.st.Store, posterOf))
}

func (w Writer) Update() schema.UpdateWriter {
	return schema.NewUpdateWriter(storage.Field(w.st.Store, updateOf))
}

func (w Writer) Home() schema.HomeWriter {
	return schema.NewHomeWriter(storage.Field(w.st.Store, homeOf))
}

func (w Writer) Weather() schema.WeatherWriter {
	return schema.NewWeatherWriter(storage.Field(w.st.Store, weatherOf))
}

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
