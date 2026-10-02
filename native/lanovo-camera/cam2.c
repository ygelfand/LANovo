#include <math.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include <android/log.h>
#include <camera/NdkCameraCaptureSession.h>
#include <camera/NdkCameraDevice.h>
#include <camera/NdkCameraManager.h>
#include <camera/NdkCameraMetadataTags.h>
#include <camera/NdkCaptureRequest.h>

#include "cam2.h"

#define TAG "lanovo-camera"
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)

#define QCOM_SATURATION 0x80070000u
#define QCOM_SHARPNESS 0x80140000u
#define OUTS 2
#define CURVE 128
#define NEUTRAL 5

struct cam2 {
	ACameraManager *mgr;
	ACameraDevice *dev;
	ACaptureRequest *req;
	ACaptureSessionOutputContainer *container;
	ACaptureSessionOutput *out[OUTS];
	ACameraOutputTarget *target[OUTS];
	ANativeWindow *win[OUTS];
	int outs;
	ACameraCaptureSession *session;
	ACameraCaptureSession_captureCallbacks results;
	pthread_mutex_t mu;
	int closing;
	int contrast, brightness;
	int32_t vendor[2];
	float base[3][CURVE];
	uint32_t basen;
};

static const uint32_t curves[3] = {ACAMERA_TONEMAP_CURVE_RED, ACAMERA_TONEMAP_CURVE_GREEN, ACAMERA_TONEMAP_CURVE_BLUE};

typedef struct {
	const char *name;
	uint8_t value;
} word;

static const word scenes[] = {{"action", 2}, {"portrait", 3}, {"landscape", 4}, {"night", 5}, {"night-portrait", 6},
	{"theatre", 7}, {"beach", 8}, {"snow", 9}, {"sunset", 10}, {"steadyphoto", 11}, {"fireworks", 12}, {"sports", 13},
	{"party", 14}, {"candlelight", 15}, {"hdr", 18}, {NULL, 0}};
static const word bandings[] = {{"off", 0}, {"50hz", 1}, {"60hz", 2}, {"auto", 3}, {NULL, 0}};
static const word balances[] = {{"auto", 1}, {"incandescent", 2}, {"fluorescent", 3}, {"warm-fluorescent", 4},
	{"daylight", 5}, {"cloudy-daylight", 6}, {"twilight", 7}, {"shade", 8}, {NULL, 0}};
static const word effects[] = {{"none", 0}, {"mono", 1}, {"negative", 2}, {"solarize", 3}, {"sepia", 4},
	{"posterize", 5}, {"whiteboard", 6}, {"blackboard", 7}, {"aqua", 8}, {NULL, 0}};

static int lookup(const word *w, const char *name, uint8_t *out) {
	for (; w->name; w++)
		if (!strcmp(w->name, name)) {
			*out = w->value;
			return 1;
		}
	return 0;
}

static void gone(void *ctx, ACameraDevice *dev) { logw("camera2: disconnected"); }
static void failed(void *ctx, ACameraDevice *dev, int err) { logw("camera2: device error %d", err); }
static void quiet(void *ctx, ACameraCaptureSession *s) {}

static ACameraDevice_StateCallbacks device_cb = {.onDisconnected = gone, .onError = failed};
static ACameraCaptureSession_stateCallbacks session_cb = {.onClosed = quiet, .onReady = quiet, .onActive = quiet};

static const char *first_id(ACameraManager *mgr, ACameraIdList **ids) {
	if (ACameraManager_getCameraIdList(mgr, ids) != ACAMERA_OK || !*ids) return NULL;
	if ((*ids)->numCameras < 1) return NULL;
	return (*ids)->cameraIds[0];
}

int cam2_present(void) {
	ACameraManager *mgr = ACameraManager_create();
	ACameraIdList *ids = NULL;
	int ok = first_id(mgr, &ids) != NULL;
	if (ids) ACameraManager_deleteCameraIdList(ids);
	ACameraManager_delete(mgr);
	return ok;
}

static void set_u8(cam2 *c, uint32_t tag, uint8_t v) {
	camera_status_t st = ACaptureRequest_setEntry_u8(c->req, tag, 1, &v);
	if (st != ACAMERA_OK) logw("camera2: tag 0x%x = %u: %d", tag, v, st);
}

static void set_i32(cam2 *c, uint32_t tag, int32_t v) {
	camera_status_t st = ACaptureRequest_setEntry_i32(c->req, tag, 1, &v);
	if (st != ACAMERA_OK) logw("camera2: tag 0x%x = %d: %d", tag, v, st);
}

static void knob(cam2 *c, const char *key, const char *value) {
	uint8_t u;
	if (!strcmp(key, "exposure-compensation")) {
		set_i32(c, ACAMERA_CONTROL_AE_EXPOSURE_COMPENSATION, atoi(value));
	} else if (!strcmp(key, "scene-mode")) {
		if (lookup(scenes, value, &u)) {
			set_u8(c, ACAMERA_CONTROL_SCENE_MODE, u);
			set_u8(c, ACAMERA_CONTROL_MODE, ACAMERA_CONTROL_MODE_USE_SCENE_MODE);
		} else {
			set_u8(c, ACAMERA_CONTROL_SCENE_MODE, ACAMERA_CONTROL_SCENE_MODE_DISABLED);
			set_u8(c, ACAMERA_CONTROL_MODE, ACAMERA_CONTROL_MODE_AUTO);
		}
	} else if (!strcmp(key, "antibanding")) {
		if (lookup(bandings, value, &u)) set_u8(c, ACAMERA_CONTROL_AE_ANTIBANDING_MODE, u);
	} else if (!strcmp(key, "whitebalance")) {
		if (lookup(balances, value, &u)) set_u8(c, ACAMERA_CONTROL_AWB_MODE, u);
	} else if (!strcmp(key, "effect")) {
		if (lookup(effects, value, &u)) set_u8(c, ACAMERA_CONTROL_EFFECT_MODE, u);
	} else if (!strcmp(key, "edge-mode")) {
		set_u8(c, ACAMERA_EDGE_MODE, (uint8_t)atoi(value));
	} else if (!strcmp(key, "noise-reduction")) {
		set_u8(c, ACAMERA_NOISE_REDUCTION_MODE, (uint8_t)atoi(value));
	} else if (!strcmp(key, "saturation")) {
		set_i32(c, QCOM_SATURATION, atoi(value));
	} else if (!strcmp(key, "sharpness")) {
		set_i32(c, QCOM_SHARPNESS, atoi(value));
	} else if (!strcmp(key, "fps-range")) {
		int32_t range[2] = {c->vendor[0], c->vendor[1]};
		if (strcmp(value, "auto") && sscanf(value, "%d,%d", &range[0], &range[1]) != 2) return;
		if (range[1] > 0) ACaptureRequest_setEntry_i32(c->req, ACAMERA_CONTROL_AE_TARGET_FPS_RANGE, 2, range);
	} else if (!strcmp(key, "contrast")) {
		c->contrast = atoi(value);
	} else if (!strcmp(key, "brightness")) {
		c->brightness = atoi(value);
	}
}

static float shaped(float y, int contrast, int brightness) {
	y = powf(y, exp2f(-(brightness - NEUTRAL) * 0.12f));
	float k = (contrast - NEUTRAL) / (float)NEUTRAL;
	if (k > 0) y += k * 0.4f * (y * y * (3 - 2 * y) - y);
	else y += -k * 0.3f * (0.5f - y);
	return y < 0 ? 0 : y > 1 ? 1 : y;
}

static void tonemap(cam2 *c) {
	if (c->contrast == NEUTRAL && c->brightness == NEUTRAL) {
		set_u8(c, ACAMERA_TONEMAP_MODE, ACAMERA_TONEMAP_MODE_FAST);
		return;
	}
	if (!c->basen) return;
	float curve[CURVE];
	for (int ch = 0; ch < 3; ch++) {
		for (uint32_t i = 0; i < c->basen; i += 2) {
			curve[i] = c->base[ch][i];
			curve[i + 1] = shaped(c->base[ch][i + 1], c->contrast, c->brightness);
		}
		camera_status_t st = ACaptureRequest_setEntry_float(c->req, curves[ch], c->basen, curve);
		if (st != ACAMERA_OK) logw("camera2: tone curve %d: %d", ch, st);
	}
	set_u8(c, ACAMERA_TONEMAP_MODE, ACAMERA_TONEMAP_MODE_CONTRAST_CURVE);
}

static int repeat(cam2 *c) {
	camera_status_t st = ACameraCaptureSession_setRepeatingRequest(c->session, &c->results, 1, &c->req, NULL);
	if (st != ACAMERA_OK) {
		logw("camera2: repeating request: %d", st);
		return -1;
	}
	return 0;
}

static int learn(cam2 *c, const ACameraMetadata *m) {
	ACameraMetadata_const_entry e;
	if (ACameraMetadata_getConstEntry(m, ACAMERA_TONEMAP_MODE, &e) != ACAMERA_OK || e.count < 1 ||
		e.data.u8[0] != ACAMERA_TONEMAP_MODE_FAST)
		return 0;
	uint32_t n = 0;
	for (int ch = 0; ch < 3; ch++) {
		if (ACameraMetadata_getConstEntry(m, curves[ch], &e) != ACAMERA_OK || e.count < 4 || e.count > CURVE ||
			(e.count & 1) || (ch && e.count != n))
			return 0;
		n = e.count;
		memcpy(c->base[ch], e.data.f, n * sizeof(float));
	}
	c->basen = n;
	return 1;
}

static void completed(void *ctx, ACameraCaptureSession *s, ACaptureRequest *r, const ACameraMetadata *m) {
	cam2 *c = ctx;
	pthread_mutex_lock(&c->mu);
	if (!c->closing && !c->basen && learn(c, m) && (c->contrast != NEUTRAL || c->brightness != NEUTRAL)) {
		tonemap(c);
		repeat(c);
	}
	pthread_mutex_unlock(&c->mu);
}

static void knobs(cam2 *c, const char *all) {
	if (!all) return;
	char *copy = strdup(all);
	for (char *save = NULL, *kv = strtok_r(copy, ";", &save); kv; kv = strtok_r(NULL, ";", &save)) {
		char *eq = strchr(kv, '=');
		if (!eq || eq == kv) continue;
		*eq = 0;
		knob(c, kv, eq + 1);
	}
	free(copy);
}

cam2 *cam2_open(ANativeWindow **outs, int n, const char *all) {
	if (n > OUTS) return NULL;
	cam2 *c = calloc(1, sizeof *c);
	c->contrast = c->brightness = NEUTRAL;
	c->results.context = c;
	c->results.onCaptureCompleted = completed;
	pthread_mutex_init(&c->mu, NULL);
	c->mgr = ACameraManager_create();
	ACameraIdList *ids = NULL;
	const char *id = first_id(c->mgr, &ids);
	camera_status_t st = ACAMERA_ERROR_UNKNOWN;
	for (int tries = 0; id && tries < 5 && !c->dev; tries++) {
		if (tries) usleep(200000);
		st = ACameraManager_openCamera(c->mgr, id, &device_cb, &c->dev);
	}
	if (ids) ACameraManager_deleteCameraIdList(ids);
	if (!c->dev) {
		logw("camera2: open: %d", st);
		goto fail;
	}
	if ((st = ACameraDevice_createCaptureRequest(c->dev, TEMPLATE_PREVIEW, &c->req)) != ACAMERA_OK) {
		logw("camera2: request: %d", st);
		goto fail;
	}
	ACaptureSessionOutputContainer_create(&c->container);
	for (int i = 0; i < n; i++) {
		c->win[i] = outs[i];
		ANativeWindow_acquire(outs[i]);
		c->outs = i + 1;
		if (ACaptureSessionOutput_create(outs[i], &c->out[i]) != ACAMERA_OK ||
			ACaptureSessionOutputContainer_add(c->container, c->out[i]) != ACAMERA_OK ||
			ACameraOutputTarget_create(outs[i], &c->target[i]) != ACAMERA_OK ||
			ACaptureRequest_addTarget(c->req, c->target[i]) != ACAMERA_OK) {
			logw("camera2: output %d", i);
			goto fail;
		}
	}
	if ((st = ACameraDevice_createCaptureSession(c->dev, c->container, &session_cb, &c->session)) != ACAMERA_OK) {
		logw("camera2: session: %d", st);
		goto fail;
	}
	ACameraMetadata_const_entry e;
	if (ACaptureRequest_getConstEntry(c->req, ACAMERA_CONTROL_AE_TARGET_FPS_RANGE, &e) == ACAMERA_OK && e.count == 2) {
		c->vendor[0] = e.data.i32[0];
		c->vendor[1] = e.data.i32[1];
	}
	if (cam2_apply(c, all) != 0) goto fail;
	return c;
fail:
	cam2_close(c);
	return NULL;
}

int cam2_apply(cam2 *c, const char *all) {
	pthread_mutex_lock(&c->mu);
	knobs(c, all);
	tonemap(c);
	int r = repeat(c);
	pthread_mutex_unlock(&c->mu);
	return r;
}

void cam2_close(cam2 *c) {
	if (!c) return;
	pthread_mutex_lock(&c->mu);
	c->closing = 1;
	pthread_mutex_unlock(&c->mu);
	if (c->session) {
		ACameraCaptureSession_stopRepeating(c->session);
		ACameraCaptureSession_close(c->session);
	}
	if (c->dev) ACameraDevice_close(c->dev);
	for (int i = 0; i < c->outs; i++) {
		if (c->target[i]) {
			if (c->req) ACaptureRequest_removeTarget(c->req, c->target[i]);
			ACameraOutputTarget_free(c->target[i]);
		}
		if (c->out[i]) {
			if (c->container) ACaptureSessionOutputContainer_remove(c->container, c->out[i]);
			ACaptureSessionOutput_free(c->out[i]);
		}
		if (c->win[i]) ANativeWindow_release(c->win[i]);
	}
	if (c->req) ACaptureRequest_free(c->req);
	if (c->container) ACaptureSessionOutputContainer_free(c->container);
	if (c->mgr) ACameraManager_delete(c->mgr);
	pthread_mutex_destroy(&c->mu);
	free(c);
}
