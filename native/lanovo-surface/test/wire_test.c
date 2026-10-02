#include <stdio.h>
#include <string.h>

#include "protocol.h"
#include "wire.h"

static int failures;

#define CHECK(cond)                                                    \
	do {                                                               \
		if (!(cond)) {                                                 \
			fprintf(stderr, "%s:%d: %s\n", __FILE__, __LINE__, #cond); \
			failures++;                                                \
		}                                                              \
	} while (0)

static void payloads_are_padded_to_words(void) {
	CHECK(wire_bytes(28, 20, 5));
	CHECK(wire_bytes(28, 20, 8));
	CHECK(wire_bytes(20, 20, 0));
	CHECK(!wire_bytes(24, 20, 5));
	CHECK(!wire_bytes(32, 20, 5));
}

static void a_huge_payload_count_does_not_wrap_into_a_short_message(void) {
	CHECK(!wire_bytes(20, 20, 0xfffffffe));
	CHECK(!wire_bytes(20, 20, 0xfffffffd));
	CHECK(!wire_bytes(28, 28, 0xffffffff));
}

static void item_counts_multiply_without_wrapping(void) {
	CHECK(wire_items(12 + 3 * 16, 12, 3, 16));
	CHECK(!wire_items(12 + 3 * 16, 12, 4, 16));
	CHECK(!wire_items(12, 12, 0x10000000, 16));
	CHECK(!wire_items(4, 4, 0x08000000, 32));
	CHECK(!wire_items(20, 20, 0x40000000, 4));
}

static void a_crypto_header_reads_in_order(void) {
	uint32_t w[CRYPT_WORDS + 4 + 2];
	memset(w, 0, sizeof w);
	w[0] = CRYPT_CENC;
	w[1] = 1;
	w[2] = 9;
	w[3] = 2;
	for (int i = 0; i < 16; i++) {
		((uint8_t *)(w + 4))[i] = (uint8_t)i;
		((uint8_t *)(w + 8))[i] = (uint8_t)(0x80 + i);
	}
	w[12] = 5, w[13] = 11, w[14] = 2, w[15] = 0;
	drm_sample s;
	const uint32_t *after = wire_crypt(w, CRYPT_WORDS + 4 + 2, &s);
	CHECK(after == w + CRYPT_WORDS + 4);
	CHECK(s.mode == CRYPT_CENC && s.pattern_encrypt == 1 && s.pattern_skip == 9 && s.subsamples == 2);
	CHECK(s.key[0] == 0 && s.key[15] == 15 && s.iv[0] == 0x80 && s.iv[15] == 0x8f);
	CHECK(s.sizes == w + CRYPT_WORDS && s.sizes[0] == 5 && s.sizes[1] == 11 && s.sizes[2] == 2);
}

static void a_crypto_header_that_overruns_is_refused(void) {
	uint32_t w[CRYPT_WORDS + 2];
	memset(w, 0, sizeof w);
	drm_sample s;
	CHECK(wire_crypt(w, CRYPT_WORDS - 1, &s) == NULL);
	w[3] = 2;
	CHECK(wire_crypt(w, CRYPT_WORDS + 2, &s) == NULL);
	w[3] = 1;
	CHECK(wire_crypt(w, CRYPT_WORDS + 2, &s) == w + CRYPT_WORDS + 2);
	w[3] = MAX_SUBSAMPLES + 1;
	CHECK(wire_crypt(w, CRYPT_WORDS + 2, &s) == NULL);
	w[3] = 0xffffffff;
	CHECK(wire_crypt(w, CRYPT_WORDS + 2, &s) == NULL);
}

static void a_data_reply_counts_its_padding(void) {
	uint32_t head[2 + 8];
	uint32_t words[3] = {7, 0, 5};
	size_t pad;
	size_t n = wire_head(head, OP_DRM_REQUEST, words, 3, 5, &pad);
	CHECK(n == 20);
	CHECK(pad == 3);
	CHECK(head[0] == OP_DRM_REQUEST && head[1] == 3 * 4 + 5 + 3);
	CHECK(head[2] == 7 && head[3] == 0 && head[4] == 5);
	wire_head(head, OP_AUDIO_SAMPLE, words, 3, 8, &pad);
	CHECK(pad == 0 && head[1] == 3 * 4 + 8);
}

int main(void) {
	payloads_are_padded_to_words();
	a_huge_payload_count_does_not_wrap_into_a_short_message();
	item_counts_multiply_without_wrapping();
	a_crypto_header_reads_in_order();
	a_crypto_header_that_overruns_is_refused();
	a_data_reply_counts_its_padding();
	if (failures) {
		fprintf(stderr, "%d failed\n", failures);
		return 1;
	}
	printf("ok\n");
	return 0;
}
