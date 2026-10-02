#include <stdlib.h>
#include <string.h>

#include <android/log.h>
#include <media/NdkMediaFormat.h>

#include "audio.h"
#include "protocol.h"

#define TAG "lanovo-surface"
#define logi(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)

#define AAC "audio/mp4a-latm"
#define INPUT_WAIT_US 20000
#define OUTPUT_WAIT_US 2000

int audio_open(audio *a, AMediaCrypto *crypto, uint32_t rate, uint32_t channels, const uint8_t *config, uint32_t config_len) {
	memset(a, 0, sizeof *a);
	AMediaCodec *c = AMediaCodec_createDecoderByType(AAC);
	if (!c) return ERR_CODEC;
	AMediaFormat *f = AMediaFormat_new();
	AMediaFormat_setString(f, AMEDIAFORMAT_KEY_MIME, AAC);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_SAMPLE_RATE, (int32_t)rate);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_CHANNEL_COUNT, (int32_t)channels);
	if (config_len) AMediaFormat_setBuffer(f, "csd-0", (void *)config, config_len);
	int err = AMediaCodec_configure(c, f, NULL, crypto, 0);
	AMediaFormat_delete(f);
	if (err || AMediaCodec_start(c)) {
		logw("audio: configure/start %d", err);
		AMediaCodec_delete(c);
		return ERR_CODEC;
	}
	a->codec = c;
	a->crypto = crypto;
	a->secure = crypto != NULL;
	a->rate = (int32_t)rate;
	a->channels = (int32_t)channels;
	logi("audio %s %u Hz x %u%s", AAC, rate, channels, crypto ? ", encrypted" : "");
	return OK;
}

static int keep(audio *a, size_t *have, const uint8_t *src, size_t n) {
	if (*have + n > a->cap) {
		size_t cap = a->cap ? a->cap : 32768;
		while (cap < *have + n) cap *= 2;
		uint8_t *p = realloc(a->pcm, cap);
		if (!p) return ERR_MEMORY;
		a->pcm = p;
		a->cap = cap;
	}
	memcpy(a->pcm + *have, src, n);
	*have += n;
	return OK;
}

static int drain(audio *a, size_t *have, int64_t wait) {
	for (;;) {
		AMediaCodecBufferInfo info;
		ssize_t idx = AMediaCodec_dequeueOutputBuffer(a->codec, &info, wait);
		if (idx == AMEDIACODEC_INFO_OUTPUT_FORMAT_CHANGED) {
			AMediaFormat *of = AMediaCodec_getOutputFormat(a->codec);
			AMediaFormat_getInt32(of, AMEDIAFORMAT_KEY_SAMPLE_RATE, &a->rate);
			AMediaFormat_getInt32(of, AMEDIAFORMAT_KEY_CHANNEL_COUNT, &a->channels);
			AMediaFormat_delete(of);
			continue;
		}
		if (idx == AMEDIACODEC_INFO_OUTPUT_BUFFERS_CHANGED) continue;
		if (idx < 0) return OK;
		size_t cap = 0;
		uint8_t *buf = AMediaCodec_getOutputBuffer(a->codec, idx, &cap);
		int st = OK;
		if (buf && info.size > 0) st = keep(a, have, buf + info.offset, info.size);
		AMediaCodec_releaseOutputBuffer(a->codec, idx, 0);
		if (st != OK) return st;
		wait = 0;
	}
}

int audio_sample(audio *a, int64_t pts, uint32_t flags, const drm_sample *crypt, const uint8_t *data, uint32_t len, size_t *pcm_len) {
	*pcm_len = 0;
	if (!a->codec) return ERR_ARGS;
	int st = drain(a, pcm_len, 0);
	if (st != OK) return st;
	ssize_t idx = AMediaCodec_dequeueInputBuffer(a->codec, INPUT_WAIT_US);
	if (idx < 0) return ERR_AGAIN;
	size_t cap = 0;
	uint8_t *buf = AMediaCodec_getInputBuffer(a->codec, idx, &cap);
	if (!buf || len > cap) {
		AMediaCodec_queueInputBuffer(a->codec, idx, 0, 0, pts, 0);
		return ERR_ARGS;
	}
	memcpy(buf, data, len);
	uint32_t cf = flags & SAMPLE_END ? AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM : 0;
	int err;
	if (crypt && crypt->mode != CRYPT_CLEAR) {
		if (!a->secure) return ERR_ARGS;
		AMediaCodecCryptoInfo *info = drm_info(crypt, len);
		if (!info) return ERR_MEMORY;
		err = AMediaCodec_queueSecureInputBuffer(a->codec, idx, 0, info, pts, cf);
		AMediaCodecCryptoInfo_delete(info);
	} else {
		err = AMediaCodec_queueInputBuffer(a->codec, idx, 0, len, pts, cf);
	}
	if (err) {
		logw("audio: queue %d", err);
		if (crypt && crypt->mode != CRYPT_CLEAR) drm_describe("audio", crypt, len);
		return ERR_CODEC;
	}
	return drain(a, pcm_len, OUTPUT_WAIT_US);
}

void audio_close(audio *a) {
	if (a->codec) {
		AMediaCodec_stop(a->codec);
		AMediaCodec_delete(a->codec);
	}
	free(a->pcm);
	memset(a, 0, sizeof *a);
}
