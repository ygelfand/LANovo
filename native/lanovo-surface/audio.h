#include <stddef.h>
#include <stdint.h>

#include <media/NdkMediaCodec.h>

#include "drm.h"

typedef struct {
	AMediaCodec *codec;
	AMediaCrypto *crypto;
	int secure;
	int32_t rate, channels;
	uint8_t *pcm;
	size_t cap;
} audio;

int audio_open(audio *a, AMediaCrypto *crypto, uint32_t rate, uint32_t channels, const uint8_t *config, uint32_t config_len);
int audio_sample(audio *a, int64_t pts, uint32_t flags, const drm_sample *crypt, const uint8_t *data, uint32_t len, size_t *pcm_len);
void audio_close(audio *a);
