package visual

func noise(i, j int) float64 {
	h := uint32(i)*0x9e3779b1 ^ uint32(j)*0x85ebca77
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	return float64(h&0xffffff) / 0xffffff
}

func onset(last *uint64, now uint64) bool {
	hit := now != *last
	*last = now
	return hit
}
