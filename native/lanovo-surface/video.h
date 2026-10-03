#include <pthread.h>
#include <stdint.h>

#include <android/native_window.h>
#include <media/NdkMediaCodec.h>

#include "drm.h"

typedef struct {
	AMediaCodec *codec;
	AMediaCrypto *crypto;
	ANativeWindow *win;
	uint32_t kind, w, h;
	char decoder[128];
	pthread_t out, maker;
	int has_out, has_maker;
	pthread_mutex_t mu, park_mu;
	pthread_cond_t park_cv;
	int64_t media_us, mono_us;
	int running;
	volatile int closing, ended, ready, failed, flushing, parked;
	volatile uint32_t gen, shown, dropped;
} video;

int video_open(video *v, ANativeWindow *win, uint32_t codec, uint32_t w, uint32_t h, AMediaCrypto *crypto, const char *decoder);
int video_sample(video *v, int64_t pts, uint32_t flags, const drm_sample *crypt, const uint8_t *data, uint32_t len);
int video_flush(video *v);
void video_clock(video *v, int64_t media_us, int running);
void video_close(video *v);
