package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/layout"
)

type Config struct {
	schema.Shared

	Device     Device     `json:"-"`
	Volume     Volume     `json:"volume"`
	Microphone Microphone `json:"microphone"`
	Diag       Diag       `json:"diag"`
	Access     Access     `json:"access"`
	Presence   Presence   `json:"presence"`
}

func Defaults() Config {
	c := Config{
		Shared:     schema.DefaultShared(),
		Volume:     defaultVolume(),
		Microphone: defaultMicrophone(),
		Diag:       defaultDiag(),
		Access:     defaultAccess(),
		Presence:   defaultPresence(),
	}
	c.Visual.Label = layout.Manufacturer
	return c
}

type Device struct {
	Name  string
	Addr  string
	Model string
}

type Writer struct {
	schema.SharedWriter[Config]
	st *Store
}

func (w Writer) Volume() VolumeWriter         { return VolumeWriter{st: w.st} }
func (w Writer) Microphone() MicrophoneWriter { return MicrophoneWriter{st: w.st} }
func (w Writer) Diag() DiagWriter             { return DiagWriter{st: w.st} }
func (w Writer) Presence() PresenceWriter     { return PresenceWriter{st: w.st} }

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
