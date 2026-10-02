#pragma once

#include <stddef.h>
#include <stdint.h>

#define MAX_SUBSAMPLES 1024

typedef struct {
	uint32_t mode, pattern_encrypt, pattern_skip, subsamples;
	uint8_t key[16], iv[16];
	const uint32_t *sizes;
} drm_sample;

int wire_bytes(uint32_t len, uint64_t head, uint64_t n);
int wire_items(uint32_t len, uint64_t head, uint64_t n, uint64_t size);
const uint32_t *wire_crypt(const uint32_t *w, uint32_t words, drm_sample *s);
size_t wire_head(uint32_t *head, uint32_t op, const uint32_t *words, uint32_t n, size_t data, size_t *pad);
