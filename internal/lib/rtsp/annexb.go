// Package rtsp serves H.264 over RTSP and RTP: interleaved on the RTSP connection or unicast UDP.
package rtsp

const (
	nalIDR = 5
	nalSPS = 7
	nalPPS = 8
	nalAUD = 9
)

// NALUs splits an Annex-B byte stream into NAL units, without their start codes.
func NALUs(b []byte) [][]byte {
	var out [][]byte
	start := -1
	i := 0
	for i+2 < len(b) {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			if start >= 0 {
				out = append(out, trimZeros(b[start:i]))
			}
			i += 3
			start = i
			continue
		}
		i++
	}
	if start >= 0 && start < len(b) {
		out = append(out, b[start:])
	}
	kept := out[:0]
	for _, n := range out {
		if len(n) > 0 {
			kept = append(kept, n)
		}
	}
	return kept
}

func trimZeros(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}

func nalType(n []byte) byte { return n[0] & 0x1f }
