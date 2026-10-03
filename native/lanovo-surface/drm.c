#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include <android/log.h>

#include "drm.h"
#include "protocol.h"

#define TAG "lanovo-surface"
#define logi(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)

#define MAX_SESSIONS 4

static drm_session sessions[MAX_SESSIONS];

static drm_session *find(uint32_t id) {
	for (int i = 0; i < MAX_SESSIONS; i++)
		if (sessions[i].used && sessions[i].id == id) return &sessions[i];
	return NULL;
}

int drm_open(uint32_t id, const uint8_t uuid[16], int privacy, const uint8_t *cert, uint32_t cert_len) {
	if (find(id)) return ERR_ARGS;
	drm_session *s = NULL;
	for (int i = 0; i < MAX_SESSIONS && !s; i++)
		if (!sessions[i].used) s = &sessions[i];
	if (!s) return ERR_FULL;
	if (!AMediaDrm_isCryptoSchemeSupported(uuid, NULL)) return ERR_DRM;
	AMediaDrm *d = AMediaDrm_createByUUID(uuid);
	if (!d) return ERR_DRM;
	AMediaDrmSessionId sid;
	media_status_t st = AMediaDrm_openSession(d, &sid);
	if (st != AMEDIA_OK) {
		logw("drm %u: open session %d", id, st);
		AMediaDrm_release(d);
		return ERR_DRM;
	}
	if (privacy) {
		AMediaDrm_setPropertyString(d, "privacyMode", "enable");
		if (cert_len && AMediaDrm_setPropertyByteArray(d, "serviceCertificate", cert, cert_len) != AMEDIA_OK)
			logw("drm %u: the service certificate was refused", id);
	}
	memset(s, 0, sizeof *s);
	s->used = 1;
	s->id = id;
	s->drm = d;
	s->sid = sid;
	memcpy(s->uuid, uuid, 16);
	logi("drm %u: session open", id);
	return OK;
}

int drm_request(uint32_t id, const uint8_t *init, uint32_t init_len, const uint8_t **out, size_t *out_len) {
	drm_session *s = find(id);
	if (!s) return ERR_ARGS;
	*out = NULL;
	*out_len = 0;
	media_status_t st = AMediaDrm_getKeyRequest(s->drm, &s->sid, init, init_len, "video/mp4", KEY_TYPE_STREAMING, NULL, 0, out, out_len);
	if (st != AMEDIA_OK) {
		logw("drm %u: key request %d", id, st);
		return ERR_DRM;
	}
	return OK;
}

int drm_provide(uint32_t id, const uint8_t *license, uint32_t len) {
	drm_session *s = find(id);
	if (!s) return ERR_ARGS;
	AMediaDrmKeySetId ks = {0};
	media_status_t st = AMediaDrm_provideKeyResponse(s->drm, &s->sid, license, len, &ks);
	if (st != AMEDIA_OK) {
		logw("drm %u: key response %d", id, st);
		return ERR_DRM;
	}
	AMediaDrmKeyValue kv[16];
	size_t n = 16;
	if (AMediaDrm_queryKeyStatus(s->drm, &s->sid, kv, &n) == AMEDIA_OK)
		for (size_t i = 0; i < n; i++) logi("drm %u: %s = %s", id, kv[i].mKey, kv[i].mValue);
	return OK;
}

static AMediaDrm *provisioning;

int drm_provision(const uint8_t uuid[16], const uint8_t **req, size_t *req_len, const char **url) {
	*req = NULL;
	*req_len = 0;
	*url = NULL;
	if (provisioning) AMediaDrm_release(provisioning);
	provisioning = AMediaDrm_isCryptoSchemeSupported(uuid, NULL) ? AMediaDrm_createByUUID(uuid) : NULL;
	if (!provisioning) return ERR_DRM;
	*req_len = 1;
	media_status_t st = AMediaDrm_getProvisionRequest(provisioning, req, req_len, url);
	if (st != AMEDIA_OK) {
		logw("drm: provision request %d", st);
		*req_len = 0;
		return ERR_DRM;
	}
	return OK;
}

int drm_provisioned(const uint8_t *resp, uint32_t len) {
	if (!provisioning) return ERR_ARGS;
	media_status_t st = AMediaDrm_provideProvisionResponse(provisioning, resp, len);
	AMediaDrm_release(provisioning);
	provisioning = NULL;
	if (st != AMEDIA_OK) {
		logw("drm: provision response %d", st);
		return ERR_DRM;
	}
	logi("drm: provisioned");
	return OK;
}

void drm_close(uint32_t id) {
	drm_session *s = find(id);
	if (!s) return;
	if (s->crypto) AMediaCrypto_delete(s->crypto);
	AMediaDrm_closeSession(s->drm, &s->sid);
	AMediaDrm_release(s->drm);
	memset(s, 0, sizeof *s);
	logi("drm %u: closed", id);
}

void drm_close_all(void) {
	for (int i = 0; i < MAX_SESSIONS; i++)
		if (sessions[i].used) drm_close(sessions[i].id);
}

AMediaCrypto *drm_crypto(uint32_t id) {
	drm_session *s = find(id);
	if (!s) return NULL;
	if (!s->crypto) {
		s->crypto = AMediaCrypto_new(s->uuid, s->sid.ptr, s->sid.length);
		if (!s->crypto) logw("drm %u: no crypto for the session", id);
	}
	return s->crypto;
}

AMediaCrypto *drm_existing(uint32_t id) {
	drm_session *s = find(id);
	return s ? s->crypto : NULL;
}

void drm_describe(const char *who, const drm_sample *s, uint32_t len) {
	char key[33], iv[33];
	for (int i = 0; i < 16; i++) {
		snprintf(key + 2 * i, 3, "%02x", s->key[i]);
		snprintf(iv + 2 * i, 3, "%02x", s->iv[i]);
	}
	logw("%s: refused sample of %u bytes, mode %u key %s iv %s subsamples %u first [%u %u]", who, len, s->mode, key, iv,
		s->subsamples, s->subsamples ? s->sizes[0] : 0, s->subsamples ? s->sizes[1] : len);
}

AMediaCodecCryptoInfo *drm_info(const drm_sample *s, uint32_t len) {
	uint32_t n = s->subsamples ? s->subsamples : 1;
	size_t *clear = calloc(n, sizeof *clear), *enc = calloc(n, sizeof *enc);
	if (!clear || !enc) {
		free(clear);
		free(enc);
		return NULL;
	}
	if (s->subsamples) {
		for (uint32_t i = 0; i < n; i++) {
			clear[i] = s->sizes[2 * i];
			enc[i] = s->sizes[2 * i + 1];
		}
	} else {
		enc[0] = len;
	}
	cryptoinfo_mode_t mode = s->mode == CRYPT_CBCS ? AMEDIACODECRYPTOINFO_MODE_AES_CBC : AMEDIACODECRYPTOINFO_MODE_AES_CTR;
	AMediaCodecCryptoInfo *info = AMediaCodecCryptoInfo_new((int)n, (uint8_t *)s->key, (uint8_t *)s->iv, mode, clear, enc);
	if (info && s->mode == CRYPT_CBCS) {
		cryptoinfo_pattern_t p = {(int32_t)s->pattern_encrypt, (int32_t)s->pattern_skip};
		AMediaCodecCryptoInfo_setPattern(info, &p);
	}
	free(clear);
	free(enc);
	return info;
}
