#include <string.h>

#include "protocol.h"
#include "wire.h"

int wire_bytes(uint32_t len, uint64_t head, uint64_t n) {
	return (uint64_t)len == head + ((n + 3) & ~(uint64_t)3);
}

int wire_items(uint32_t len, uint64_t head, uint64_t n, uint64_t size) {
	return n <= UINT32_MAX && (uint64_t)len == head + n * size;
}

const uint32_t *wire_crypt(const uint32_t *w, uint32_t words, drm_sample *s) {
	if (words < CRYPT_WORDS) return NULL;
	s->mode = w[0];
	s->pattern_encrypt = w[1];
	s->pattern_skip = w[2];
	s->subsamples = w[3];
	memcpy(s->key, w + 4, 16);
	memcpy(s->iv, w + 8, 16);
	if (s->subsamples > MAX_SUBSAMPLES || words < CRYPT_WORDS + 2 * s->subsamples) return NULL;
	s->sizes = w + CRYPT_WORDS;
	return w + CRYPT_WORDS + 2 * s->subsamples;
}

size_t wire_head(uint32_t *head, uint32_t op, const uint32_t *words, uint32_t n, size_t data, size_t *pad) {
	*pad = (4 - data % 4) % 4;
	head[0] = op;
	head[1] = (uint32_t)(n * 4 + data + *pad);
	memcpy(head + 2, words, n * 4);
	return (2 + (size_t)n) * 4;
}
