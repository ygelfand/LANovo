package recording

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/lib/wave"
)

// somewhere points the store at a directory of this test's own, and sets how many recordings each
// assistant keeps.
func somewhere(t *testing.T, keep ...int) *Store {
	t.Helper()

	was := dir
	dir = t.TempDir()
	t.Cleanup(func() { dir = was })

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	for slot, n := range keep {
		if err := config.Set().Wake(slot).Recordings(n); err != nil {
			t.Fatalf("setting slot %d: %v", slot+1, err)
		}
	}
	return &Store{}
}

func record(t *testing.T, s *Store, id string, slot int, pcm []byte) {
	t.Helper()

	s.Opens(id, slot)
	s.Frame(pcm)
	s.Closes()
}

func kept(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func has(names []string, name string) bool {
	for _, got := range names {
		if got == name {
			return true
		}
	}
	return false
}

// An assistant set to keep two holds the two most recent and lets the rest go, audio and sidecar
// together. Recordings are the one thing here that grows without bound if nothing sweeps.
func TestOnlyTheNewestRecordingsAreKept(t *testing.T) {
	s := somewhere(t, 2)

	for _, id := range []string{"first", "second", "third"} {
		record(t, s, id, 0, []byte{1, 2, 3, 4})
	}

	got := kept(t)
	for _, name := range []string{"second.wav", "second.json", "third.wav", "third.json"} {
		if !has(got, name) {
			t.Errorf("%s was pruned, and it is one of the newest two", name)
		}
	}
	for _, name := range []string{"first.wav", "first.json"} {
		if has(got, name) {
			t.Errorf("%s survived, and it is the oldest of three", name)
		}
	}
}

// Two turns of the same second have to order by which came first, and an id cannot say: they are
// Home Assistant's and sort however they sort. Kept in seconds, the pair ties and the survivor is
// whichever the tie-break happened to favour rather than the one that was said last.
func TestTheLastTurnSurvivesEvenWithinASecond(t *testing.T) {
	s := somewhere(t, 1)

	record(t, s, "zz-earlier", 0, []byte{1, 2})
	record(t, s, "aa-later", 0, []byte{3, 4})

	got := kept(t)
	if !has(got, "aa-later.wav") {
		t.Error("the turn that came last was pruned")
	}
	if has(got, "zz-earlier.wav") {
		t.Error("the turn that came first survived over a newer one")
	}
}

// One assistant's limit says nothing about another's.
func TestEachAssistantIsPrunedAgainstItsOwnLimit(t *testing.T) {
	s := somewhere(t, 0, 1)

	record(t, s, "kept", 1, []byte{1, 2})
	record(t, s, "never", 0, []byte{1, 2})

	got := kept(t)
	if !has(got, "kept.wav") {
		t.Error("the recording of an assistant set to keep one was pruned")
	}
	if has(got, "never.wav") {
		t.Error("an assistant set to keep none recorded anyway")
	}
}

// A turn whose sidecar never arrived is a crash caught halfway, and it cannot be pruned by slot
// because nothing says which slot it was.
func TestAnOrphanedRecordingIsSweptUp(t *testing.T) {
	s := somewhere(t, 2)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "halfway.wav"), []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}

	s.Prune()

	if has(kept(t), "halfway.wav") {
		t.Error("a recording with no sidecar was left behind")
	}
}

// A turn that gathered no audio is not a recording, and saving it would put a file in the list that
// plays nothing.
func TestAnEmptyTurnIsNotSaved(t *testing.T) {
	s := somewhere(t, 2)

	s.Opens("silent", 0)
	s.Closes()

	if got := kept(t); len(got) != 0 {
		t.Errorf("an empty turn left %v behind", got)
	}
}

// A pipeline that never closes the run holds the microphone open, and the buffer is not the place to
// find out how long that lasted.
func TestATurnStopsGrowingAtTheCap(t *testing.T) {
	s := somewhere(t, 1)

	s.Opens("long", 0)
	for range 4 {
		s.Frame(make([]byte, Longest/2))
	}
	s.Closes()

	if got := s.Seconds("long"); got > 31 {
		t.Errorf("a turn was kept for %.0fs, want the cap of %ds", got, Longest/(mic.Voice*2))
	}
}

// The header has to say what the audio is: 16-bit mono at the voice rate, and a data size that
// matches what follows it. A player reads this and nothing else.
func TestTheHeaderDescribesTheAudio(t *testing.T) {
	pcm := make([]byte, 320)
	got := wave.Mono16(pcm, mic.Voice)

	if string(got[:4]) != "RIFF" || string(got[8:12]) != "WAVE" {
		t.Fatalf("not a wav: %q", got[:12])
	}
	if n := binary.LittleEndian.Uint16(got[22:24]); n != 1 {
		t.Errorf("%d channels, want mono", n)
	}
	if rate := binary.LittleEndian.Uint32(got[24:28]); rate != mic.Voice {
		t.Errorf("%d Hz, want the voice rate", rate)
	}
	if bits := binary.LittleEndian.Uint16(got[34:36]); bits != 16 {
		t.Errorf("%d bits a sample, want 16", bits)
	}
	if size := binary.LittleEndian.Uint32(got[40:44]); int(size) != len(pcm) {
		t.Errorf("the header says %d bytes of audio and there are %d", size, len(pcm))
	}
	if len(got) != wave.Header+len(pcm) {
		t.Errorf("%d bytes for a %d byte recording", len(got), len(pcm))
	}
}
