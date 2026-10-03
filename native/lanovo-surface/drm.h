#pragma once

#include <stdint.h>

#include <media/NdkMediaCodec.h>
#include <media/NdkMediaCrypto.h>
#include <media/NdkMediaDrm.h>

#include "wire.h"

typedef struct {
	int used;
	uint32_t id;
	AMediaDrm *drm;
	AMediaDrmSessionId sid;
	AMediaCrypto *crypto;
	uint8_t uuid[16];
} drm_session;

int drm_open(uint32_t id, const uint8_t uuid[16], int privacy, const uint8_t *cert, uint32_t cert_len);
int drm_request(uint32_t id, const uint8_t *init, uint32_t init_len, const uint8_t **out, size_t *out_len);
int drm_provide(uint32_t id, const uint8_t *license, uint32_t len);
int drm_provision(const uint8_t uuid[16], const uint8_t **req, size_t *req_len, const char **url);
int drm_provisioned(const uint8_t *resp, uint32_t len);
void drm_close(uint32_t id);
void drm_close_all(void);
AMediaCrypto *drm_crypto(uint32_t id);
AMediaCrypto *drm_existing(uint32_t id);
void drm_describe(const char *who, const drm_sample *s, uint32_t len);
AMediaCodecCryptoInfo *drm_info(const drm_sample *s, uint32_t len);
