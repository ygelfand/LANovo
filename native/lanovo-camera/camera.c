#include <errno.h>
#include <poll.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/uio.h>
#include <sys/un.h>
#include <unistd.h>

#include <android/log.h>
#include <android/native_window.h>
#include <media/NdkMediaCodec.h>
#include <media/NdkMediaFormat.h>

#include "cam2.h"
#include "protocol.h"
#include "turn.h"

#define TAG "lanovo-camera"
#define logi(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)

#define COLOR_FORMAT_SURFACE 0x7F000789
#define AVC_PROFILE_BASELINE 0x01
#define AVC_PROFILE_HIGH 0x08
#define AVC_LEVEL_31 0x200
#define AVC_LEVEL_51 0x4000
#define USER_SWITCHED 1
#define VIDEO_BUFFER_QUEUE 2
#define MAIN 0
#define SUB 1

extern void ps_self(void **sret) __asm__("_ZN7android12ProcessState4selfEv");
extern void ps_start_pool(void *self) __asm__("_ZN7android12ProcessState15startThreadPoolEv");
extern void *s16_ctor(void *self, const char *s) __asm__("_ZN7android8String16C1EPKc");
extern void *s8_ctor(void *self, const char *s) __asm__("_ZN7android7String8C1EPKc");
extern void cam_service(void **sret) __asm__("_ZN7android10CameraBaseINS_6CameraENS_12CameraTraitsIS1_EEE16getCameraServiceEv");
extern void cam_event(void *sret, void *svc, int event, const void *args)
	__asm__("_ZN7android8hardware15BpCameraService17notifySystemEventEiRKNSt3__16vectorIiNS2_9allocatorIiEEEE");
extern void cam_connect(void **sret, int id, const void *pkg, int uid, int pid) __asm__("_ZN7android6Camera7connectEiRKNS_8String16Eii");
extern void cam_params(void **sret, const void *cam) __asm__("_ZNK7android6Camera13getParametersEv");
extern int cam_set_params(void *cam, const void *params) __asm__("_ZN7android6Camera13setParametersERKNS_7String8E");
extern int cam_target(void *cam, const void *gbp) __asm__("_ZN7android6Camera16setPreviewTargetERKNS_2spINS_22IGraphicBufferProducerEEE");
extern int cam_start(void *cam) __asm__("_ZN7android6Camera12startPreviewEv");
extern void cam_stop(void *cam) __asm__("_ZN7android6Camera11stopPreviewEv");
extern int cam_video_mode(void *cam, int mode) __asm__("_ZN7android6Camera18setVideoBufferModeEi");
extern int cam_video_target(void *cam, const void *gbp) __asm__("_ZN7android6Camera14setVideoTargetERKNS_2spINS_22IGraphicBufferProducerEEE");
extern int cam_record(void *cam) __asm__("_ZN7android6Camera14startRecordingEv");
extern void cam_unrecord(void *cam) __asm__("_ZN7android6Camera13stopRecordingEv");
extern void cam_disconnect(void *cam) __asm__("_ZN7android10CameraBaseINS_6CameraENS_12CameraTraitsIS1_EEE10disconnectEv");
extern void surface_gbp(void **sret, const void *surface) __asm__("_ZNK7android7Surface25getIGraphicBufferProducerEv");

typedef struct {
	AMediaCodec *codec[2];
	ANativeWindow *win[2];
	void *cam;
	cam2 *c2;
	int recording;
	uint32_t w, h;
	int still;
	char *extra;
	char range[32];
	turner *turn;
} session;

static int read_full(int fd, void *buf, size_t len) {
	char *p = buf;
	while (len) {
		ssize_t r = read(fd, p, len);
		if (r < 0 && errno == EINTR) continue;
		if (r <= 0) return -1;
		p += r;
		len -= r;
	}
	return 0;
}

static int send_all(int fd, struct iovec *iov, int n, size_t total) {
	struct msghdr msg = {0};
	msg.msg_iov = iov;
	msg.msg_iovlen = n;
	while (total) {
		ssize_t w = sendmsg(fd, &msg, MSG_NOSIGNAL);
		if (w < 0 && errno == EINTR) continue;
		if (w <= 0) return -1;
		total -= w;
		while (w > 0 && msg.msg_iovlen) {
			size_t take = (size_t)w < msg.msg_iov->iov_len ? (size_t)w : msg.msg_iov->iov_len;
			msg.msg_iov->iov_base = (char *)msg.msg_iov->iov_base + take;
			msg.msg_iov->iov_len -= take;
			w -= take;
			if (!msg.msg_iov->iov_len) {
				msg.msg_iov++;
				msg.msg_iovlen--;
			}
		}
	}
	return 0;
}

static char *with(const char *params, const char *key, const char *value) {
	size_t n = strlen(params) + strlen(key) + strlen(value) + 2;
	char *out = malloc(n);
	const char *at = strstr(params, key);
	if (!at || (at != params && at[-1] != ';')) {
		snprintf(out, n, "%s;%s%s", params, key, value);
		return out;
	}
	const char *end = strchr(at, ';');
	snprintf(out, n, "%.*s%s%s%s", (int)(at - params), params, key, value, end ? end : "");
	return out;
}

static char *merged(const char *params, const char *extra) {
	char *out = strdup(params);
	char *copy = strdup(extra);
	for (char *save = NULL, *kv = strtok_r(copy, ";", &save); kv; kv = strtok_r(NULL, ";", &save)) {
		char *eq = strchr(kv, '=');
		if (!eq || eq == kv) continue;
		char key[128];
		size_t n = (size_t)(eq - kv) + 1;
		if (n >= sizeof key) continue;
		memcpy(key, kv, n);
		key[n] = 0;
		char *next = with(out, key, eq + 1);
		free(out);
		out = next;
	}
	free(copy);
	return out;
}

static char *camera1(session *s, const char *extra) {
	const char *at = strstr(extra, "fps-range=");
	if (!at || (at != extra && at[-1] != ';')) return strdup(extra);
	const char *v = at + strlen("fps-range=");
	char range[32];
	int lo, hi;
	if (!strncmp(v, "auto", 4)) snprintf(range, sizeof range, "%s", s->range);
	else if (sscanf(v, "%d,%d", &lo, &hi) == 2) snprintf(range, sizeof range, "%d,%d", lo * 1000, hi * 1000);
	else range[0] = 0;
	const char *end = strchr(at, ';');
	size_t n = strlen(extra) + sizeof range + 32;
	char *out = malloc(n);
	snprintf(out, n, "%.*s%s%s%s", (int)(at - extra), extra, range[0] ? "preview-fps-range=" : "", range,
		end ? end : "");
	return out;
}

static int apply(session *s, const char *raw) {
	void *got = NULL;
	cam_params(&got, s->cam);
	char *extra = camera1(s, raw);
	char *all = merged(got ? (const char *)got : "", extra);
	void *params = NULL;
	s8_ctor(&params, all);
	int r = cam_set_params(s->cam, &params);
	if (r != 0) logw("setParameters %s: %d", extra, r);
	free(all);
	free(extra);
	return r;
}

static void sizes(const char *params) {
	static int said;
	if (said++) return;
	size_t n = strlen(params);
	for (size_t at = 0; at < n; at += 900) logi("params %zu: %.900s", at / 900, params + at);
}

static void close_session(session *s) {
	cam2_close(s->c2);
	s->c2 = NULL;
	if (s->cam) {
		if (s->recording) cam_unrecord(s->cam);
		cam_stop(s->cam);
		cam_disconnect(s->cam);
		s->cam = NULL;
	}
	turn_close(s->turn);
	s->turn = NULL;
	for (int i = 0; i < 2; i++) {
		if (s->codec[i]) {
			AMediaCodec_stop(s->codec[i]);
			AMediaCodec_delete(s->codec[i]);
			s->codec[i] = NULL;
		}
		if (s->win[i]) {
			ANativeWindow_release(s->win[i]);
			s->win[i] = NULL;
		}
	}
}

static int producer(ANativeWindow *win, void **gbp) {
	// ANativeWindow sits after RefBase, Surface's primary base.
	*gbp = NULL;
	surface_gbp(gbp, (char *)win - 2 * sizeof(void *));
	return *gbp ? 0 : -1;
}

static int open_encoder(session *s, int at, uint32_t w, uint32_t h, uint32_t fps, uint32_t bitrate, uint32_t keyframe) {
	AMediaCodec **codec = &s->codec[at];
	*codec = AMediaCodec_createEncoderByType("video/avc");
	if (!*codec) return ERR_ENCODER;
	AMediaFormat *f = AMediaFormat_new();
	AMediaFormat_setString(f, AMEDIAFORMAT_KEY_MIME, "video/avc");
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_WIDTH, (int32_t)w);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_HEIGHT, (int32_t)h);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_COLOR_FORMAT, COLOR_FORMAT_SURFACE);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_BIT_RATE, (int32_t)bitrate);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_FRAME_RATE, (int32_t)fps);
	AMediaFormat_setInt32(f, AMEDIAFORMAT_KEY_I_FRAME_INTERVAL, (int32_t)keyframe);
	AMediaFormat_setInt32(f, "profile", AVC_PROFILE_HIGH);
	AMediaFormat_setInt32(f, "level", AVC_LEVEL_51);
	media_status_t st = AMediaCodec_configure(*codec, f, NULL, NULL, AMEDIACODEC_CONFIGURE_FLAG_ENCODE);
	if (st != AMEDIA_OK) {
		logw("encoder refused high profile: %d", st);
		AMediaCodec_delete(*codec);
		*codec = AMediaCodec_createEncoderByType("video/avc");
		if (!*codec) {
			AMediaFormat_delete(f);
			return ERR_ENCODER;
		}
		AMediaFormat_setInt32(f, "profile", AVC_PROFILE_BASELINE);
		AMediaFormat_setInt32(f, "level", AVC_LEVEL_31);
		st = AMediaCodec_configure(*codec, f, NULL, NULL, AMEDIACODEC_CONFIGURE_FLAG_ENCODE);
	}
	AMediaFormat_delete(f);
	if (st != AMEDIA_OK) {
		logw("encoder configure %dx%d@%u: %d", w, h, fps, st);
		return ERR_ENCODER;
	}
	if (AMediaCodec_createInputSurface(*codec, &s->win[at]) != AMEDIA_OK || !s->win[at]) return ERR_ENCODER;
	if (AMediaCodec_start(*codec) != AMEDIA_OK) return ERR_ENCODER;
	return OK;
}

static int open_camera(session *s, uint32_t w, uint32_t h, uint32_t fps, uint32_t sw, uint32_t sh, uint32_t quarters) {
	ANativeWindow *target = s->win[MAIN];
	if (quarters != NO_TURN) {
		uint32_t cw = (quarters & 1) ? h : w, ch = (quarters & 1) ? w : h;
		s->turn = turn_open(s->win[MAIN], (int)w, (int)h, sw ? s->win[SUB] : NULL, (int)sw, (int)sh, (int)cw, (int)ch,
			(int)quarters, &target);
		if (!s->turn) return ERR_CAMERA;
		w = cw;
		h = ch;
		sw = sh = 0;
	}
	void *svc = NULL;
	cam_service(&svc);
	if (!svc) return ERR_CAMERA;
	int users[1] = {0};
	int *vec[3] = {users, users + 1, users + 1};
	int status[8] = {0};
	cam_event(status, svc, USER_SWITCHED, vec);

	if (cam2_present()) {
		ANativeWindow *outs[2] = {target, s->win[SUB]};
		s->c2 = cam2_open(outs, sw ? 2 : 1, s->extra);
		return s->c2 ? OK : ERR_CAMERA;
	}

	void *pkg = NULL;
	s16_ctor(&pkg, "lanovo");
	for (int tries = 0; tries < 5 && !s->cam; tries++) {
		if (tries) usleep(200000);
		cam_connect(&s->cam, 0, &pkg, -1, -1);
	}
	if (!s->cam) return ERR_CAMERA;

	void *got = NULL;
	cam_params(&got, s->cam);
	sizes(got ? (const char *)got : "");
	const char *was = got ? strstr((const char *)got, "preview-fps-range=") : NULL;
	if (was && (was == (const char *)got || was[-1] == ';'))
		sscanf(was + strlen("preview-fps-range="), "%31[0-9,]", s->range);
	char size[32];
	snprintf(size, sizeof size, "%ux%u", w, h);
	char *c = with(got ? (const char *)got : "", "preview-size=", size);
	if (sw) {
		char video[32];
		snprintf(video, sizeof video, "%ux%u", sw, sh);
		char *d = with(c, "video-size=", video);
		char *e = with(d, "recording-hint=", "true");
		free(c);
		free(d);
		c = e;
	}
	if (s->extra && *s->extra) {
		char *x = camera1(s, s->extra);
		char *e = merged(c, x);
		free(x);
		free(c);
		c = e;
	}
	void *params = NULL;
	s8_ctor(&params, c);
	int r = cam_set_params(s->cam, &params);
	free(c);
	if (r != 0) {
		logw("setParameters %ux%u@%u sub %ux%u extra %s: %d", w, h, fps, sw, sh, s->extra ? s->extra : "", r);
		return ERR_ARGS;
	}

	void *gbp = NULL;
	if (producer(target, &gbp) != 0 || cam_target(s->cam, &gbp) != 0) return ERR_CAMERA;
	if (sw) {
		void *sub = NULL;
		if ((r = cam_video_mode(s->cam, VIDEO_BUFFER_QUEUE)) != 0) {
			logw("video buffer queue mode: %d", r);
			return ERR_CAMERA;
		}
		if (producer(s->win[SUB], &sub) != 0 || (r = cam_video_target(s->cam, &sub)) != 0) {
			logw("video target: %d", r);
			return ERR_CAMERA;
		}
	}
	if (cam_start(s->cam) != 0) return ERR_CAMERA;
	if (sw) {
		if ((r = cam_record(s->cam)) != 0) {
			logw("startRecording: %d", r);
			return ERR_CAMERA;
		}
		s->recording = 1;
	}
	return OK;
}

static void request_key(session *s) {
	AMediaFormat *f = AMediaFormat_new();
	AMediaFormat_setInt32(f, "request-sync", 0);
	for (int i = 0; i < 2; i++) {
		if (!s->codec[i]) continue;
		media_status_t st = AMediaCodec_setParameters(s->codec[i], f);
		if (st != AMEDIA_OK) logw("request-sync on stream %d: %d", i, st);
	}
	AMediaFormat_delete(f);
}

static int gone(int conn, session *s) {
	struct pollfd p = {.fd = conn, .events = POLLIN};
	if (poll(&p, 1, 0) <= 0) return 0;
	if (p.revents & (POLLHUP | POLLERR)) return 1;
	char c;
	if (recv(conn, &c, 1, MSG_DONTWAIT) <= 0) return 1;
	if (c == ASK_PARAMS) {
		uint32_t n = 0;
		if (read_full(conn, &n, sizeof n) < 0 || n > MAX_PARAMS) return 1;
		char *extra = calloc(1, n + 1);
		if (read_full(conn, extra, n) < 0) {
			free(extra);
			return 1;
		}
		if (s->c2) {
			char *all = merged(s->extra ? s->extra : "", extra);
			free(s->extra);
			s->extra = all;
			cam2_apply(s->c2, s->extra);
		} else if (s->cam) {
			apply(s, extra);
		}
		free(extra);
		return 0;
	}
	if (c == ASK_STILL) s->still = 1;
	if (c == ASK_KEY) request_key(s);
	return 0;
}

static int send_still(int conn, session *s, unsigned char *rgba) {
	s->still = 0;
	size_t n = rgba ? (size_t)s->w * s->h * 4 : 0;
	if (!rgba) logw("still: nothing to read back");
	uint32_t head[FRAME_WORDS] = {(uint32_t)n, FRAME_STILL, s->w, s->h};
	struct iovec iov[2] = {{head, sizeof head}, {rgba, n}};
	int err = send_all(conn, iov, rgba ? 2 : 1, sizeof head + n);
	free(rgba);
	return err;
}

static int drain(int conn, session *s, int at, int64_t wait, uint64_t *frames) {
	AMediaCodecBufferInfo info;
	ssize_t i = AMediaCodec_dequeueOutputBuffer(s->codec[at], &info, wait);
	if (i < 0) return 0;
	size_t cap = 0;
	uint8_t *buf = AMediaCodec_getOutputBuffer(s->codec[at], i, &cap);
	int err = 0;
	if (buf && info.size > 0) {
		uint32_t flags = at == SUB ? FRAME_SUB : 0;
		if (info.flags & AMEDIACODEC_BUFFER_FLAG_CODEC_CONFIG) flags |= FRAME_CONFIG;
		if (info.flags & 1) flags |= FRAME_KEY;

		uint64_t pts = (uint64_t)info.presentationTimeUs;
		uint32_t head[FRAME_WORDS] = {(uint32_t)info.size, flags, (uint32_t)pts, (uint32_t)(pts >> 32)};
		struct iovec iov[2] = {{head, sizeof head}, {buf + info.offset, (size_t)info.size}};
		err = send_all(conn, iov, 2, sizeof head + (size_t)info.size);
		frames[at]++;

	}
	AMediaCodec_releaseOutputBuffer(s->codec[at], i, false);
	return err;
}

static void stream(int conn, session *s) {
	uint64_t frames[2] = {0, 0};
	int64_t wait = s->codec[SUB] ? 10000 : 100000;
	if (s->turn) wait = 5000;
	while (!gone(conn, s)) {
		if (s->turn) {
			unsigned char *still = NULL;
			int drew = turn_frame(s->turn, s->still ? &still : NULL);
			if (drew && s->still && send_still(conn, s, still)) break;
		} else if (s->still && send_still(conn, s, NULL)) {
			break;
		}
		if (drain(conn, s, MAIN, wait, frames)) break;
		if (s->codec[SUB] && drain(conn, s, SUB, wait, frames)) break;
	}
	logi("client gone after %llu main and %llu sub frames", (unsigned long long)frames[MAIN],
		(unsigned long long)frames[SUB]);
}

static void serve(int conn) {
	uint32_t req[OPEN_WORDS];
	if (read_full(conn, req, sizeof req) < 0 || req[0] != MAGIC) return;

	uint32_t w = req[2], h = req[3], fps = req[4], bitrate = req[5], keyframe = req[6];
	uint32_t sw = req[7], sh = req[8], sbitrate = req[9], quarters = req[10];
	uint32_t st = OK;
	session s = {0};
	uint32_t n = 0;
	if (req[1] == VERSION) {
		if (read_full(conn, &n, sizeof n) < 0 || n > MAX_PARAMS) return;
		s.extra = calloc(1, n + 1);
		if (read_full(conn, s.extra, n) < 0) {
			free(s.extra);
			return;
		}
	}
	if (req[1] != VERSION) st = ERR_VERSION;
	else if (!w || !h || !fps || !bitrate || w > 4096 || h > 4096 || fps > 120) st = ERR_ARGS;
	else if (sw && (!sh || !sbitrate || sw > 4096 || sh > 4096)) st = ERR_ARGS;
	s.w = w;
	s.h = h;
	if (st == OK) st = (uint32_t)open_encoder(&s, MAIN, w, h, fps, bitrate, keyframe ? keyframe : 1);
	if (st == OK && sw) st = (uint32_t)open_encoder(&s, SUB, sw, sh, fps, sbitrate, keyframe ? keyframe : 1);
	if (st == OK) st = (uint32_t)open_camera(&s, w, h, fps, sw, sh, quarters);

	uint32_t reply[REPLY_WORDS] = {st, w, h, fps};
	struct iovec iov = {reply, sizeof reply};
	if (send_all(conn, &iov, 1, sizeof reply) == 0 && st == OK) {
		logi("streaming %ux%u@%u at %u bps, sub %ux%u at %u bps, turn %d", w, h, fps, bitrate, sw, sh, sbitrate,
			quarters == NO_TURN ? -1 : (int)quarters);
		stream(conn, &s);
	} else if (st != OK) {
		logw("open %ux%u@%u failed: %u", w, h, fps, st);
	}
	close_session(&s);
	free(s.extra);
}

static int listener(void) {
	const char *env = getenv("ANDROID_SOCKET_" SOCKET_NAME);
	if (env) return atoi(env);
	int s = socket(AF_UNIX, SOCK_STREAM, 0);
	struct sockaddr_un addr = {.sun_family = AF_UNIX};
	strncpy(addr.sun_path, SOCKET_PATH, sizeof addr.sun_path - 1);
	unlink(SOCKET_PATH);
	if (bind(s, (struct sockaddr *)&addr, sizeof addr) < 0) return -1;
	chmod(SOCKET_PATH, 0666);
	return s;
}

int main(void) {
	signal(SIGPIPE, SIG_IGN);
	int lsock = listener();
	if (lsock < 0 || listen(lsock, 2) < 0) {
		logw("socket %s: %s", SOCKET_PATH, strerror(errno));
		return 1;
	}
	void *ps = NULL;
	ps_self(&ps);
	ps_start_pool(ps);
	logi("ready on %s", SOCKET_PATH);
	for (;;) {
		int conn = accept(lsock, NULL, NULL);
		if (conn < 0) continue;
		serve(conn);
		close(conn);
	}
}
