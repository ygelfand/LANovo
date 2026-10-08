package wave

const Header = 44

func Mono16(pcm []byte, rate int) []byte { return PCM16(pcm, rate, 1) }

func PCM16(pcm []byte, rate, channels int) []byte {
	out := make([]byte, 0, Header+len(pcm))

	put32 := func(v uint32) {
		out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	}
	put16 := func(v uint16) { out = append(out, byte(v), byte(v>>8)) }

	out = append(out, "RIFF"...)
	put32(uint32(Header - 8 + len(pcm)))
	out = append(out, "WAVEfmt "...)
	put32(16) // the size of the format chunk that follows
	put16(1)  // PCM, uncompressed
	put16(uint16(channels))
	put32(uint32(rate))
	put32(uint32(rate * 2 * channels))
	put16(uint16(2 * channels))
	put16(16) // bits a sample
	out = append(out, "data"...)
	put32(uint32(len(pcm)))

	return append(out, pcm...)
}
