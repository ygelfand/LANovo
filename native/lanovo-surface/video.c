#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#include <android/log.h>
#include <media/NdkMediaCodec.h>
#include <media/NdkMediaFormat.h>

#include "protocol.h"
#include "video.h"

#define TAG "lanovo-surface"
#define logi(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)

#define EARLY_US 4000
#define LATE_US 40000

static int64_t mono_us(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (int64_t)ts.tv_sec * 1000000 + ts.tv_nsec / 1000;
}

static const char *mime_of(uint32_t codec) {
	switch (codec) {
	case CODEC_VP9: return "video/x-vnd.on2.vp9";
	case CODEC_AVC: return "video/avc";
	case CODEC_HEVC: return "video/hevc";
	case CODEC_VP8: return "video/x-vnd.on2.vp8";
	}
	return NULL;
}

static const char *secure_name(uint32_t codec) {
	switch (codec) {
	case CODEC_VP9: return "OMX.qcom.video.decoder.vp9.secure";
	case CODEC_AVC: return "OMX.qcom.video.decoder.avc.secure";
	case CODEC_HEVC: return "OMX.qcom.video.decoder.hevc.secure";
	}
	return NULL;
}

static int64_t media_now(video *v, int *running) {
	pthread_mutex_lock(&v->mu);
	int64_t at = v->media_us;
	if (v->running) at += mono_us() - v->mono_us;
	*running = v->running;
	pthread_mutex_unlock(&v->mu);
	return at;
}

static void park(video *v) {
	pthread_mutex_lock(&v->park_mu);
	v->parked = 1;
	pthread_cond_broadcast(&v->park_cv);
	while (v->flushing && !v->closing) pthread_cond_wait(&v->park_cv, &v->park_mu);
	v->parked = 0;
	pthread_mutex_unlock(&v->park_mu);
}

static void *output(void *arg) {
	video *v = arg;
	while (!v->closing) {
		if (v->flushing) {
			park(v);
			continue;
		}
		AMediaCodecBufferInfo info;
		ssize_t idx = AMediaCodec_dequeueOutputBuffer(v->codec, &info, 10000);
		if (idx < 0) continue;
		if (info.flags & AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM) {
			AMediaCodec_releaseOutputBuffer(v->codec, idx, 0);
			v->ended = 1;
			continue;
		}
		int render = 1, abandon = 0;
		for (;;) {
			if (v->closing || v->flushing) { abandon = 1; break; }
			int running;
			int64_t now = media_now(v, &running);
			if (info.presentationTimeUs < now - LATE_US) { render = 0; break; }
			if (v->shown == 0) break;
			if (running && info.presentationTimeUs <= now + EARLY_US) break;
			usleep(2000);
		}
		if (abandon) continue;
		AMediaCodec_releaseOutputBuffer(v->codec, idx, render);
		if (render) v->shown++;
		else v->dropped++;
	}
	return NULL;
}

static void *make(void *arg) {
	video *v = arg;
	const char *mime = mime_of(v->kind);
	const char *secure = v->crypto && AMediaCrypto_requiresSecureDecoderComponent(mime) ? secure_name(v->kind) : NULL;
	AMediaCodec *c = secure ? AMediaCodec_createCodecByName(secure) : AMediaCodec_createDecoderByType(mime);
	if (!c) {
		logw("no decoder for %s%s", mime, secure ? " (secure)" : "");
		v->failed = 1;
		return NULL;
	}
	AMediaFormat *f = AMediaFormat_new();
	AMediaFormat_setString(f, AMEDIAFORMAT_KEY_MIME, mime);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_WIDTH, v->w);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_HEIGHT, v->h);
	int err = AMediaCodec_configure(c, f, v->win, v->crypto, 0);
	AMediaFormat_delete(f);
	if (err || AMediaCodec_start(c)) {
		logw("%s: configure/start %d", mime, err);
		AMediaCodec_delete(c);
		v->failed = 1;
		return NULL;
	}
	v->codec = c;
	if (v->closing) return NULL;
	pthread_create(&v->out, NULL, output, v);
	v->has_out = 1;
	v->ready = 1;
	logi("video %s %ux%u%s", mime, v->w, v->h, secure ? " secure" : "");
	return NULL;
}

int video_open(video *v, ANativeWindow *win, uint32_t codec, uint32_t w, uint32_t h, AMediaCrypto *crypto) {
	if (!mime_of(codec)) return ERR_ARGS;
	memset(v, 0, sizeof *v);
	v->win = win;
	v->crypto = crypto;
	v->kind = codec;
	v->w = w;
	v->h = h;
	pthread_mutex_init(&v->mu, NULL);
	pthread_mutex_init(&v->park_mu, NULL);
	pthread_cond_init(&v->park_cv, NULL);
	pthread_create(&v->maker, NULL, make, v);
	v->has_maker = 1;
	return OK;
}

int video_sample(video *v, int64_t pts, uint32_t flags, const drm_sample *crypt, const uint8_t *data, uint32_t len) {
	if (v->failed) return ERR_CODEC;
	if (!v->ready) return ERR_AGAIN;
	ssize_t idx = AMediaCodec_dequeueInputBuffer(v->codec, 5000);
	if (idx < 0) return ERR_AGAIN;
	size_t cap = 0;
	uint8_t *buf = AMediaCodec_getInputBuffer(v->codec, idx, &cap);
	int st;
	if (!buf || len > cap) {
		AMediaCodec_queueInputBuffer(v->codec, idx, 0, 0, pts, 0);
		st = ERR_ARGS;
	} else {
		memcpy(buf, data, len);
		uint32_t cf = flags & SAMPLE_END ? AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM : 0;
		if (crypt && crypt->mode != CRYPT_CLEAR) {
			AMediaCodecCryptoInfo *info = v->crypto ? drm_info(crypt, len) : NULL;
			if (!info) {
				AMediaCodec_queueInputBuffer(v->codec, idx, 0, 0, pts, 0);
				return v->crypto ? ERR_MEMORY : ERR_ARGS;
			}
			st = AMediaCodec_queueSecureInputBuffer(v->codec, idx, 0, info, pts, cf) == 0 ? OK : ERR_CODEC;
			AMediaCodecCryptoInfo_delete(info);
			if (st != OK) drm_describe("video", crypt, len);
		} else {
			st = AMediaCodec_queueInputBuffer(v->codec, idx, 0, len, pts, cf) == 0 ? OK : ERR_CODEC;
		}
	}
	return st;
}

int video_flush(video *v) {
	if (v->failed) return ERR_CODEC;
	if (!v->ready) return OK;
	pthread_mutex_lock(&v->park_mu);
	v->flushing = 1;
	while (!v->parked && !v->closing) pthread_cond_wait(&v->park_cv, &v->park_mu);
	int err = AMediaCodec_flush(v->codec);
	v->gen++;
	v->shown = v->dropped = 0;
	v->ended = 0;
	pthread_mutex_lock(&v->mu);
	v->media_us = 0;
	v->running = 0;
	pthread_mutex_unlock(&v->mu);
	v->flushing = 0;
	pthread_cond_broadcast(&v->park_cv);
	pthread_mutex_unlock(&v->park_mu);
	return err ? ERR_CODEC : OK;
}

void video_clock(video *v, int64_t media_us, int running) {
	pthread_mutex_lock(&v->mu);
	v->media_us = media_us;
	v->mono_us = mono_us();
	v->running = running;
	pthread_mutex_unlock(&v->mu);
}

void video_close(video *v) {
	pthread_mutex_lock(&v->park_mu);
	v->closing = 1;
	pthread_cond_broadcast(&v->park_cv);
	pthread_mutex_unlock(&v->park_mu);
	if (v->has_maker) pthread_join(v->maker, NULL);
	if (v->has_out) pthread_join(v->out, NULL);
	if (v->codec) {
		AMediaCodec_stop(v->codec);
		AMediaCodec_delete(v->codec);
		v->codec = NULL;
	}
	pthread_mutex_destroy(&v->mu);
	pthread_mutex_destroy(&v->park_mu);
	pthread_cond_destroy(&v->park_cv);
	v->has_maker = v->has_out = 0;
}
