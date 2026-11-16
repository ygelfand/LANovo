package config

type Speaker struct {
	EQ bool `json:"eq"`
}

func defaultSpeaker() Speaker { return Speaker{EQ: true} }
