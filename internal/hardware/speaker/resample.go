package speaker

const (
	VoiceRate     = 16000
	VoiceUpsample = Rate / VoiceRate
)

const voiceHeadroom = 0.9

type sinc struct{ up *Rational }

func newSinc() *sinc {
	up := NewRational(VoiceRate, Rate, 1)
	up.Headroom(voiceHeadroom)
	return &sinc{up: up}
}

func (u *sinc) Reset()          { u.up.Reset() }
func (u *sinc) Clipped() uint64 { return u.up.Clipped() }

func (u *sinc) Run(mono []int16, out []int16) []int16 {
	for _, v := range u.up.Run(mono) {
		out = append(out, v, v)
	}
	return out
}
