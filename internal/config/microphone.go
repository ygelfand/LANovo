package config

type Microphone struct {
	// The decimators' digital gain, 0..124.
	Gain int `json:"gain"`

	Sensitivity int `json:"sensitivity"`

	Leveling bool `json:"leveling"`

	Denoise bool `json:"denoise"`

	VisualizerLift int `json:"visualizerLift"`
}

const (
	// Where the vendor's HAL leaves the decimators.
	DefaultMicGain = 84

	DefaultMicSensitivity = 8

	DefaultLeveling = true

	DefaultDenoise = true

	DefaultVisualizerLift = 10
)

func defaultMicrophone() Microphone {
	return Microphone{
		Gain:        DefaultMicGain,
		Sensitivity: DefaultMicSensitivity,
		Leveling:    DefaultLeveling,
		Denoise:     DefaultDenoise,

		VisualizerLift: DefaultVisualizerLift,
	}
}

func (w MicrophoneWriter) VisualizerLift(db int) error {
	return w.st.Update(func(c *Config) { c.Microphone.VisualizerLift = db })
}

type MicrophoneWriter struct{ st *Store }

func (w MicrophoneWriter) Gain(v int) error {
	return w.st.Update(func(c *Config) { c.Microphone.Gain = v })
}

func (w MicrophoneWriter) Sensitivity(db int) error {
	return w.st.Update(func(c *Config) { c.Microphone.Sensitivity = db })
}

func (w MicrophoneWriter) Denoise(on bool) error {
	return w.st.Update(func(c *Config) { c.Microphone.Denoise = on })
}

func (w MicrophoneWriter) Leveling(on bool) error {
	return w.st.Update(func(c *Config) { c.Microphone.Leveling = on })
}
