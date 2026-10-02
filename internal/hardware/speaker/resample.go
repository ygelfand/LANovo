package speaker

// What a voice pipeline sends, and what that makes of each sample at the rate the card takes.
const (
	VoiceRate     = 16000
	VoiceUpsample = Rate / VoiceRate
)

// Voice arrives at 16 kHz and the codec only takes 48 kHz, so every input sample turns into three.
// Repeating it three times is the cheap way and it sounds like it: a held sample is a zero-order
// hold, whose spectrum keeps images of the speech band mirrored around the input rate, which lands
// them in 8 kHz to 16 kHz where they are plainly audible as grit on every consonant.
//
// Interpolating with a low pass at the input's Nyquist removes them, which is what Rational does for
// every other rate on this device. Three to one is just the easy ratio.

// voiceHeadroom is how much is taken off the filter. Replies arrive at very nearly full scale and
// interpolation overshoots a transient by a few percent, so without it the result clips.
const voiceHeadroom = 0.9

// sinc turns a 16 kHz mono stream into 48 kHz stereo. It keeps the tail of the previous call, so an
// utterance delivered in chunks comes out as one continuous signal rather than with a seam at every
// boundary.
type sinc struct{ up *Rational }

func newSinc() *sinc {
	up := NewRational(VoiceRate, Rate, 1)
	up.Headroom(voiceHeadroom)
	return &sinc{up: up}
}

func (u *sinc) Reset()          { u.up.Reset() }
func (u *sinc) Clipped() uint64 { return u.up.Clipped() }

// Run appends the interleaved stereo result of mono to out.
//
// The filter runs in mono and each sample is written twice, rather than duplicating first and
// filtering two identical channels: the same answer for half the multiplies.
func (u *sinc) Run(mono []int16, out []int16) []int16 {
	for _, v := range u.up.Run(mono) {
		out = append(out, v, v)
	}
	return out
}
