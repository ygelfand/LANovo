package hls

import (
	"bytes"
	"testing"

	"github.com/bluenviron/gohlslib/v2/pkg/playlist"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
)

func rate(f float64) *float64 { return &f }

func TestPickTakesTheRichestPlayableVariantWithinLimits(t *testing.T) {
	vs := []*playlist.MultivariantVariant{
		{URI: "240", Bandwidth: 300_000, Resolution: "426x240", Codecs: []string{"avc1.4D4015", "mp4a.40.5"}},
		{URI: "480", Bandwidth: 1_200_000, Resolution: "854x480", Codecs: []string{"avc1.4D401E", "mp4a.40.2"}},
		{URI: "720", Bandwidth: 2_500_000, Resolution: "1280x720", FrameRate: rate(30), Codecs: []string{"avc1.4D401F", "mp4a.40.2"}},
		{URI: "720p60", Bandwidth: 4_000_000, Resolution: "1280x720", FrameRate: rate(60), Codecs: []string{"avc1.4D4020", "mp4a.40.2"}},
		{URI: "1440", Bandwidth: 9_000_000, Resolution: "2560x1440", FrameRate: rate(30), Codecs: []string{"avc1.640032", "mp4a.40.2"}},
		{URI: "vp9", Bandwidth: 5_000_000, Resolution: "1920x1080", Codecs: []string{"vp09.00.40.08", "mp4a.40.2"}},
	}
	if v := pick(vs, 1080, 30); v == nil || v.URI != "720" {
		t.Fatalf("picked %+v", v)
	}
	if v := pick(vs[:1], 1080, 30); v != nil {
		t.Errorf("picked HE-AAC %s", v.URI)
	}
}

func TestPackedAudioGivesItsTimestampAndFrames(t *testing.T) {
	adts, err := mpeg4audio.ADTSPackets{
		{Type: mpeg4audio.ObjectTypeAACLC, SampleRate: 48000, ChannelConfig: 2, AU: []byte{1, 2, 3}},
		{Type: mpeg4audio.ObjectTypeAACLC, SampleRate: 48000, ChannelConfig: 2, AU: []byte{4, 5}},
	}.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	priv := append([]byte(tsTimestampOwner+"\x00"), 0, 0, 0, 1, 0, 0, 0, 0x10)
	frame := append([]byte("PRIV"), 0, 0, 0, byte(len(priv)), 0, 0)
	frame = append(frame, priv...)
	tag := append([]byte("ID3\x04\x00\x00"), 0, 0, 0, byte(len(frame)))
	seg := append(append(tag, frame...), adts...)

	pts, rest, ok := id3Timestamp(seg)
	if !ok || pts != 1<<32+0x10 {
		t.Fatalf("timestamp %d, found %v", pts, ok)
	}
	var pkts mpeg4audio.ADTSPackets
	if err := pkts.Unmarshal(rest); err != nil || len(pkts) != 2 || !bytes.Equal(pkts[1].AU, []byte{4, 5}) {
		t.Fatalf("%d frames, %v", len(pkts), err)
	}
}

func TestMonoIsDoubledToStereo(t *testing.T) {
	got := stereo([]byte{1, 0, 0xff, 0xff}, 1)
	want := []int16{1, 1, -1, -1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}
