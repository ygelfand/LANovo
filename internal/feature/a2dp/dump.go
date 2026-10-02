package a2dp

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Writing what came out of the decoder and what came out of the resampler, so a fault in the audio
// can be looked at rather than argued about.
//
// Two files because the question is which half is at fault. The room already says the card and the
// mixer are fine, since anything else playing through them sounds right.

// Where the samples go. Temporary, and named so it is obvious what left them behind.
const (
	dumpFrames = "/data/local/tmp/bt-frames.sbc"
	dumpPre    = "/data/local/tmp/bt-decoded.pcm"
	dumpPost   = "/data/local/tmp/bt-resampled.pcm"
)

var dumping dump

// dump holds the files while a capture is running.
type dump struct {
	mu     sync.Mutex
	frames *os.File
	pre    *os.File
	post   *os.File
	until  time.Time
}

// Dump captures both ends of the chain for a while, and says where it put them.
func Dump(within time.Duration) (string, error) {
	dumping.mu.Lock()
	defer dumping.mu.Unlock()

	dumping.shut()

	made := make([]*os.File, 0, 3)
	for _, path := range []string{dumpFrames, dumpPre, dumpPost} {
		f, err := os.Create(path)
		if err != nil {
			for _, o := range made {
				o.Close()
			}
			return "", err
		}
		made = append(made, f)
	}

	dumping.frames, dumping.pre, dumping.post = made[0], made[1], made[2]
	dumping.until = time.Now().Add(within)

	return fmt.Sprintf("capturing %s to\n  %s  frames as they arrived\n"+
		"  %s  decoded, stereo\n  %s  resampled, stereo 48k\n",
		within, dumpFrames, dumpPre, dumpPost), nil
}

// frame takes one SBC frame exactly as it came off the link, before anything reads it.
func (d *dump) frame(b []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.frames == nil || time.Now().After(d.until) {
		return
	}
	d.frames.Write(b)
}

// write takes one buffer from each end. Nothing happens unless a capture is running.
func (d *dump) write(pre, post []int16) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.pre == nil {
		return
	}
	if time.Now().After(d.until) {
		d.shut()
		return
	}

	d.pre.Write(raw(pre))
	d.post.Write(raw(post))
}

// shut closes what is open. The caller holds the lock.
func (d *dump) shut() {
	if d.pre == nil {
		return
	}

	d.frames.Close()
	d.pre.Close()
	d.post.Close()
	d.frames, d.pre, d.post = nil, nil, nil

	slog.Info("bluetooth capture finished",
		"frames", dumpFrames, "decoded", dumpPre, "resampled", dumpPost)
}

// raw is the samples as little endian bytes, which is what every tool that reads a pcm file wants.
func raw(s []int16) []byte {
	b := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return b
}
