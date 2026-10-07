package media

import (
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/libcountertop/pkg/media/pcm"
)

var Formats = []esphome.MediaFormat{
	{Format: "wav", SampleRate: speaker.Rate, Channels: speaker.Channels, SampleBytes: 2},
	{Format: "wav", SampleRate: speaker.VoiceRate, Channels: 1, SampleBytes: 2, Announcement: true},
}

const Tail = pcm.Tail

var Fetch = pcm.Fetch
