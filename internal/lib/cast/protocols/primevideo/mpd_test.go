package primevideo

import (
	"testing"
	"time"
)

const listManifest = `<?xml version="1.0"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT1H52M36.352S">
 <Period id="intro" duration="PT6S">
  <AdaptationSet contentType="video" mimeType="video/mp4"><ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cenc" default_KID="AAAA"/>
   <Representation id="i1" bandwidth="5000000" width="1920" height="800" codecs="avc1.640028"><BaseURL>/intro/v.mp4</BaseURL>
    <SegmentBase timescale="96000" presentationTimeOffset="0" indexRange="1000-1055"><Initialization range="0-999"/></SegmentBase></Representation>
  </AdaptationSet>
  <AdaptationSet contentType="audio" mimeType="audio/mp4" lang="en"><Representation id="ia" bandwidth="128000" codecs="mp4a.40.2"><BaseURL>/intro/a.mp4</BaseURL><SegmentBase indexRange="500-555"><Initialization range="0-499"/></SegmentBase></Representation></AdaptationSet>
 </Period>
 <Period id="main" start="PT6S">
  <AdaptationSet contentType="video" mimeType="video/mp4">
   <Representation id="v1" bandwidth="100000" width="480" height="200" codecs="avc1.4D4016">
    <BaseURL>v_1.mp4</BaseURL>
    <SegmentList timescale="1000" duration="2000"><Initialization range="0-999"/><SegmentURL mediaRange="1000-1999"/><SegmentURL mediaRange="2000-3499"/></SegmentList>
   </Representation>
   <Representation id="v2" bandwidth="15000000" width="1920" height="800" codecs="avc1.640028">
    <BaseURL>v_2.mp4</BaseURL>
    <SegmentList timescale="1000" duration="2000"><Initialization range="0-1099"/><SegmentURL mediaRange="1100-5099"/><SegmentURL mediaRange="5100-9099"/></SegmentList>
   </Representation>
  </AdaptationSet>
  <AdaptationSet contentType="audio" mimeType="audio/mp4" lang="es"><Representation id="a1" bandwidth="128000" codecs="mp4a.40.2"><BaseURL>a_es.mp4</BaseURL><SegmentList timescale="1000" duration="2000"><Initialization range="0-499"/><SegmentURL mediaRange="500-999"/></SegmentList></Representation></AdaptationSet>
  <AdaptationSet contentType="audio" mimeType="audio/mp4" lang="en"><Role schemeIdUri="urn:mpeg:dash:role:2011" value="description"/><Representation id="a2" bandwidth="128000" codecs="mp4a.40.2"><BaseURL>a_en_ad.mp4</BaseURL><SegmentList timescale="1000" duration="2000"><Initialization range="0-499"/><SegmentURL mediaRange="500-999"/></SegmentList></Representation></AdaptationSet>
  <AdaptationSet contentType="audio" mimeType="audio/mp4" lang="en"><Representation id="a3" bandwidth="128000" codecs="mp4a.40.2"><BaseURL>a_en.mp4</BaseURL><SegmentList timescale="1000" duration="2000"><Initialization range="0-499"/><SegmentURL mediaRange="500-999"/></SegmentList></Representation></AdaptationSet>
  <AdaptationSet contentType="audio" mimeType="audio/mp4" lang="en"><Representation id="a4" bandwidth="640000" codecs="ec-3"><BaseURL>a_en_ddp.mp4</BaseURL><SegmentList timescale="1000" duration="2000"><Initialization range="0-499"/><SegmentURL mediaRange="500-999"/></SegmentList></Representation></AdaptationSet>
 </Period>
</MPD>`

func TestTheTallestVideoAndTheMainAudioInTheLanguageArePicked(t *testing.T) {
	parts, err := Pick([]byte(listManifest), "https://cdn.example/path/manifest.mpd?token=x", 1080, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || parts[0].ID != "intro" || parts[0].Duration != 6*time.Second || parts[1].Start != 6*time.Second {
		t.Fatalf("parts %+v", parts)
	}
	if parts[0].Video.URL != "https://cdn.example/intro/v.mp4" || parts[0].Video.KID != "AAAA" || parts[0].Video.Index != [2]int64{1000, 1055} {
		t.Fatalf("intro %+v", parts[0].Video)
	}
	v, a := parts[1].Video, parts[1].Audio
	if v.URL != "https://cdn.example/path/v_2.mp4" || v.Height != 800 || v.Init != [2]int64{0, 1099} || len(v.Frags) != 2 {
		t.Fatalf("video %+v", v)
	}
	if f := v.Frags[1]; f.Offset != 5100 || f.Size != 4000 || f.At != 2*time.Second {
		t.Fatalf("fragment %+v", f)
	}
	if a.URL != "https://cdn.example/path/a_en.mp4" {
		t.Fatalf("audio %+v", a)
	}
	if got := Length([]byte(listManifest)); got != time.Hour+52*time.Minute+36352*time.Millisecond {
		t.Fatalf("length %v", got)
	}
}

func TestOnlyALeadingClearPeriodBeforeAnEncryptedOneIsABumper(t *testing.T) {
	clear := Part{Start: 0, Duration: 6 * time.Second}
	locked := Part{Start: 6 * time.Second, Video: Rep{KID: "K"}}
	if at, ok := Bumper([]Part{clear, locked}); !ok || at != 6*time.Second {
		t.Errorf("bumper %v %v", at, ok)
	}
	for _, parts := range [][]Part{{locked}, {clear}, {locked, clear}, {clear, clear}} {
		if _, ok := Bumper(parts); ok {
			t.Errorf("%+v taken for a bumper", parts)
		}
	}
}
