package cenc

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func mbox(kind string, parts ...[]byte) []byte {
	body := bytes.Join(parts, nil)
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out, uint32(8+len(body)))
	copy(out[4:], kind)
	return append(out, body...)
}

func full(kind string, version byte, flags uint32, parts ...[]byte) []byte {
	head := []byte{version, byte(flags >> 16), byte(flags >> 8), byte(flags)}
	return mbox(kind, append([][]byte{head}, parts...)...)
}

func u16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
func u32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
func u64(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }

var (
	trackKey = [16]byte{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
	groupKey = [16]byte{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf}
	sps      = []byte{0x67, 0x4d, 0x40, 0x1f, 0x01}
	pps      = []byte{0x68, 0xee, 0x3c}
)

func tenc(kid [16]byte, ivSize byte) []byte {
	return full("tenc", 0, 0, []byte{0, 0, 1, ivSize}, kid[:])
}

func sinf(format string) []byte {
	return mbox("sinf",
		mbox("frma", []byte(format)),
		full("schm", 0, 0, []byte("cenc"), u32(0x10000)),
		mbox("schi", tenc(trackKey, 8)))
}

func videoInit(timescale uint32) []byte {
	entry := make([]byte, 78)
	binary.BigEndian.PutUint16(entry[24:], 1280)
	binary.BigEndian.PutUint16(entry[26:], 720)
	avcc := append([]byte{1, 0x4d, 0x40, 0x1f, 0xff, 0xe1}, u16(uint16(len(sps)))...)
	avcc = append(append(avcc, sps...), 1)
	avcc = append(append(avcc, u16(uint16(len(pps)))...), pps...)
	encv := mbox("encv", entry, mbox("avcC", avcc), sinf("avc1"))
	return initWith(timescale, encv)
}

func audioInit() []byte {
	entry := make([]byte, 28)
	binary.BigEndian.PutUint16(entry[16:], 2)
	binary.BigEndian.PutUint32(entry[24:], 44100<<16)
	config := []byte{0x12, 0x10}
	dec := append([]byte{0x40, 0x15}, make([]byte, 11)...)
	dec = append(append(dec, 5, byte(len(config))), config...)
	es := append([]byte{0, 1, 0, 4, byte(len(dec))}, dec...)
	esds := full("esds", 0, 0, append([]byte{3, byte(len(es))}, es...))
	enca := mbox("enca", entry, esds, sinf("mp4a"))
	return initWith(44100, enca)
}

func initWith(timescale uint32, entry []byte) []byte {
	mdhd := full("mdhd", 0, 0, u32(0), u32(0), u32(timescale), u32(0), u32(0))
	stsd := full("stsd", 0, 0, u32(1), entry)
	return append(mbox("ftyp", []byte("iso6")), mbox("moov",
		mbox("trak", mbox("mdia", mdhd, mbox("minf", mbox("stbl", stsd)))))...)
}

type fakeSample struct {
	data  []byte
	sync  bool
	iv    uint64
	clear uint16
}

func segmentOf(decode uint64, samples []fakeSample, senc, aux bool, groups []int) []byte {
	var auxData []byte
	for _, s := range samples {
		auxData = append(auxData, u64(s.iv)...)
		auxData = append(auxData, u16(1)...)
		auxData = append(auxData, u16(s.clear)...)
		auxData = append(auxData, u32(uint32(len(s.data))-uint32(s.clear))...)
	}
	build := func(dataOffset, auxOffset uint32) []byte {
		trun := []byte{}
		trun = append(trun, u32(uint32(len(samples)))...)
		trun = append(trun, u32(dataOffset)...)
		for _, s := range samples {
			flags := uint32(nonSync)
			if s.sync {
				flags = 0
			}
			trun = append(trun, u32(3003)...)
			trun = append(trun, u32(uint32(len(s.data)))...)
			trun = append(trun, u32(flags)...)
		}
		parts := [][]byte{
			full("tfhd", 0, tfhdBaseMoof, u32(1)),
			full("tfdt", 1, 0, u64(decode)),
			full("trun", 0, trunOffset|trunDuration|trunSize|trunFlags, trun),
		}
		if aux {
			sizes := []byte{0}
			sizes = append(sizes, u32(uint32(len(samples)))...)
			for range samples {
				sizes = append(sizes, 16)
			}
			parts = append(parts, full("saiz", 0, 0, sizes), full("saio", 0, 0, u32(1), u32(auxOffset)))
		}
		if senc {
			parts = append(parts, full("senc", 0, sencSubsample, u32(uint32(len(samples))), auxData))
		}
		if groups != nil {
			entry := append([]byte{0, 0, 1, 8}, groupKey[:]...)
			parts = append(parts, full("sgpd", 1, 0, []byte("seig"), u32(20), u32(1), entry))
			sb := append([]byte("seig"), u32(uint32(len(groups)))...)
			for _, g := range groups {
				sb = append(sb, u32(1)...)
				sb = append(sb, u32(uint32(g))...)
			}
			parts = append(parts, full("sbgp", 0, 0, sb))
		}
		return mbox("moof", full("mfhd", 0, 0, u32(1)), mbox("traf", parts...))
	}
	moof := build(0, 0)
	head := uint32(len(moof) + 8)
	auxOffset := head
	dataOffset := head
	if aux {
		dataOffset += uint32(len(auxData))
	}
	moof = build(dataOffset, auxOffset)
	payload := []byte{}
	if aux {
		payload = append(payload, auxData...)
	}
	for _, s := range samples {
		payload = append(payload, s.data...)
	}
	return append(moof, mbox("mdat", payload)...)
}

func nal(body ...byte) []byte { return append(u32(uint32(len(body))), body...) }

func twoPictures() []fakeSample {
	first := append(nal(0x65, 1, 2, 3, 4, 5, 6, 7), nal(0x06, 9, 9)...)
	return []fakeSample{
		{data: first, sync: true, iv: 0x0102030405060708, clear: 5},
		{data: nal(0x41, 7, 7, 7, 7, 7, 7), iv: 0x0102030405060709, clear: 5},
	}
}

func TestAVideoInitReadsItsProtectionAndParameterSets(t *testing.T) {
	tr, err := ParseInit(videoInit(90000))
	if err != nil {
		t.Fatal(err)
	}
	if tr.Format != "avc1" || tr.Width != 1280 || tr.Height != 720 || tr.Timescale != 90000 {
		t.Errorf("track %s %dx%d @%d", tr.Format, tr.Width, tr.Height, tr.Timescale)
	}
	if tr.Scheme != SchemeCENC || tr.KeyID != trackKey || tr.IVSize != 8 || tr.LengthSize != 4 {
		t.Errorf("protection %q %x iv %d len %d", tr.Scheme, tr.KeyID, tr.IVSize, tr.LengthSize)
	}
	if len(tr.SPS) != 1 || !bytes.Equal(tr.SPS[0], sps) || len(tr.PPS) != 1 || !bytes.Equal(tr.PPS[0], pps) {
		t.Errorf("parameter sets %x %x", tr.SPS, tr.PPS)
	}
}

func TestAnAudioInitReadsItsRateChannelsAndConfig(t *testing.T) {
	tr, err := ParseInit(audioInit())
	if err != nil {
		t.Fatal(err)
	}
	if tr.Format != "mp4a" || tr.Rate != 44100 || tr.Channels != 2 || !bytes.Equal(tr.Config, []byte{0x12, 0x10}) {
		t.Errorf("track %s %d Hz x %d config %x", tr.Format, tr.Rate, tr.Channels, tr.Config)
	}
}

func checkPictures(t *testing.T, samples []Sample, want []fakeSample, keys [][16]byte) {
	t.Helper()
	if len(samples) != len(want) {
		t.Fatalf("%d samples, want %d", len(samples), len(want))
	}
	for i, s := range samples {
		w := want[i]
		if !bytes.Equal(s.Data, w.data) || s.Key != w.sync || !s.Encrypted {
			t.Errorf("#%d data %x key %v encrypted %v", i, s.Data, s.Key, s.Encrypted)
		}
		var iv [16]byte
		binary.BigEndian.PutUint64(iv[:], w.iv)
		if s.IV != iv || s.KeyID != keys[i] {
			t.Errorf("#%d iv %x key %x", i, s.IV, s.KeyID)
		}
		if len(s.Subsamples) != 1 || s.Subsamples[0].Clear != uint32(w.clear) || int(s.Subsamples[0].Clear+s.Subsamples[0].Encrypted) != len(w.data) {
			t.Errorf("#%d subsamples %v", i, s.Subsamples)
		}
	}
}

func TestAuxiliaryInfoAfterTheFragmentGivesEachSampleItsIVAndSubsamples(t *testing.T) {
	tr, _ := ParseInit(videoInit(90000))
	want := twoPictures()
	samples, _, err := ParseFragment(tr, segmentOf(0, want, false, true, nil))
	if err != nil {
		t.Fatal(err)
	}
	checkPictures(t, samples, want, [][16]byte{trackKey, trackKey})
}

func TestASencBoxGivesTheSameAsAuxiliaryInfo(t *testing.T) {
	tr, _ := ParseInit(videoInit(90000))
	want := twoPictures()
	samples, _, err := ParseFragment(tr, segmentOf(0, want, true, false, nil))
	if err != nil {
		t.Fatal(err)
	}
	checkPictures(t, samples, want, [][16]byte{trackKey, trackKey})
}

func TestASampleGroupOverridesTheTracksKey(t *testing.T) {
	tr, _ := ParseInit(videoInit(90000))
	want := twoPictures()
	samples, _, err := ParseFragment(tr, segmentOf(0, want, false, true, []int{0, 1}))
	if err != nil {
		t.Fatal(err)
	}
	checkPictures(t, samples, want, [][16]byte{trackKey, groupKey})
}

func TestHoursOfLiveMediaTimeDoNotOverflow(t *testing.T) {
	tr, _ := ParseInit(videoInit(90000))
	decode := uint64(305*3600) * 90000
	samples, _, err := ParseFragment(tr, segmentOf(decode, twoPictures(), false, true, nil))
	if err != nil {
		t.Fatal(err)
	}
	if want := 305 * time.Hour; samples[0].At != want {
		t.Errorf("first sample at %v, want %v", samples[0].At, want)
	}
	if got := samples[1].At - samples[0].At; got != 3003*time.Second/90000 {
		t.Errorf("second sample %v later", got)
	}
}

func TestAnnexBKeepsTheSubsampleMapWhole(t *testing.T) {
	tr, _ := ParseInit(videoInit(90000))
	samples, _, err := ParseFragment(tr, segmentOf(0, twoPictures(), false, true, nil))
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range samples {
		out, subs, err := tr.AnnexB(s)
		if err != nil {
			t.Fatal(err)
		}
		sum := 0
		for _, x := range subs {
			sum += int(x.Clear + x.Encrypted)
		}
		if sum != len(out) {
			t.Errorf("#%d subsamples cover %d of %d bytes", i, sum, len(out))
		}
		if !bytes.HasPrefix(out, startCode) {
			t.Errorf("#%d starts %x", i, out[:4])
		}
		if s.Key != bytes.Contains(out, append(append([]byte{}, startCode...), sps...)) {
			t.Errorf("#%d key %v but parameter sets present %v", i, s.Key, !s.Key)
		}
		body := out[len(out)-len(s.Data):]
		for off := 0; off+4 <= len(body); {
			if !bytes.Equal(body[off:off+4], startCode) {
				t.Errorf("#%d NAL at %d starts %x", i, off, body[off:off+4])
				break
			}
			off += 4 + int(binary.BigEndian.Uint32(s.Data[off:]))
		}
	}
}
