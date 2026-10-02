package config

// Microphone is how hard the microphones are driven and what counts as something happening.
type Microphone struct {
	// Gain is the decimators' digital gain, 0..124. Digital, so it lifts the room's noise with
	// the speech.
	Gain int `json:"gain"`

	// Sensitivity is how far over the room's own floor the published level starts counting, in dB.
	// Relative, because an absolute threshold cannot be right: the same number is below the noise
	// in one house and above the speech in another.
	Sensitivity int `json:"sensitivity"`

	// Leveling applies gain so speech reaches recognition near the level the models were trained
	// on.
	Leveling bool `json:"leveling"`

	Denoise bool `json:"denoise"`

	VisualizerLift int `json:"visualizerLift"`
}

const (
	// Where the vendor's HAL leaves the decimators, which has headroom either way.
	DefaultMicGain = 84

	// Eight dB, which is what the leveller's mapping onto 0 to 1 was calibrated against. Not the
	// same number as speechOverFloorDB, which decides what the floor tracking treats as speech.
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

// MicrophoneWriter changes how the microphones are driven.
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
