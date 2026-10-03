#include <errno.h>
#include <math.h>
#include <pthread.h>
#include <time.h>
#include <fcntl.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/uio.h>
#include <sys/un.h>
#include <unistd.h>

#include <android/log.h>
#include <android/native_window.h>

#include "audio.h"
#include "drm.h"
#include "gl.h"
#include "protocol.h"
#include "ui.h"
#include "video.h"

#define TAG "lanovo-surface"
#define logi(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)

#define PIXEL_FORMAT_RGBA_8888 1
#define SF_SECURE 0x80
#define SF_OPAQUE 0x400
// ANDROID_NATIVE_WINDOW_MAGIC, '_wnd'.
#define WINDOW_MAGIC 0x5f776e64

extern void ps_self(void **sret) __asm__("_ZN7android12ProcessState4selfEv");
extern void ps_start_pool(void *self) __asm__("_ZN7android12ProcessState15startThreadPoolEv");
extern void *sf_client_ctor(void *self) __asm__("_ZN7android21SurfaceComposerClientC1Ev");
extern int sf_client_check(const void *self) __asm__("_ZNK7android21SurfaceComposerClient9initCheckEv");
extern void rb_incstrong(const void *self, const void *id) __asm__("_ZNK7android7RefBase9incStrongEPKv");
extern void rb_decstrong(const void *self, const void *id) __asm__("_ZNK7android7RefBase9decStrongEPKv");
extern void *s8_ctor(void *self, const char *s) __asm__("_ZN7android7String8C1EPKc");
extern void sf_create(void **sret, void *client, const void *name, uint32_t w, uint32_t h, int32_t format,
	uint32_t flags, void *parent, uint32_t wtype, uint32_t uid)
	__asm__("_ZN7android21SurfaceComposerClient13createSurfaceERKNS_7String8EjjijPNS_14SurfaceControlEjj");
extern void sf_open(void) __asm__("_ZN7android21SurfaceComposerClient21openGlobalTransactionEv");
extern void sf_close(int sync) __asm__("_ZN7android21SurfaceComposerClient22closeGlobalTransactionEb");
extern int sc_layer(void *sc, int32_t z) __asm__("_ZN7android14SurfaceControl8setLayerEi");
extern int sc_position(void *sc, float x, float y) __asm__("_ZN7android14SurfaceControl11setPositionEff");
extern int sc_size(void *sc, uint32_t w, uint32_t h) __asm__("_ZN7android14SurfaceControl7setSizeEjj");
extern int sc_alpha(void *sc, float a) __asm__("_ZN7android14SurfaceControl8setAlphaEf");
extern int sc_show(void *sc) __asm__("_ZN7android14SurfaceControl4showEv");
extern int sc_hide(void *sc) __asm__("_ZN7android14SurfaceControl4hideEv");
extern void sc_clear(void *sc) __asm__("_ZN7android14SurfaceControl5clearEv");
extern void sc_surface(void **sret, const void *sc) __asm__("_ZNK7android14SurfaceControl10getSurfaceEv");
extern int sc_matrix(void *sc, float dsdx, float dtdx, float dtdy, float dsdy) __asm__("_ZN7android14SurfaceControl9setMatrixEffff");
extern void sf_display(void **sret, int id) __asm__("_ZN7android21SurfaceComposerClient17getBuiltInDisplayEi");
extern void ss_ctor(void *self) __asm__("_ZN7android16ScreenshotClientC1Ev");
extern void ss_dtor(void *self) __asm__("_ZN7android16ScreenshotClientD1Ev");
extern int ss_update(void *self, const void *display, int32_t left, int32_t top, int32_t right, int32_t bottom, int identity)
	__asm__("_ZN7android16ScreenshotClient6updateERKNS_2spINS_7IBinderEEENS_4RectEb");
extern const void *ss_pixels(const void *self) __asm__("_ZNK7android16ScreenshotClient9getPixelsEv");
extern uint32_t ss_width(const void *self) __asm__("_ZNK7android16ScreenshotClient8getWidthEv");
extern uint32_t ss_height(const void *self) __asm__("_ZNK7android16ScreenshotClient9getHeightEv");
extern uint32_t ss_stride(const void *self) __asm__("_ZNK7android16ScreenshotClient9getStrideEv");
extern int32_t ss_format(const void *self) __asm__("_ZNK7android16ScreenshotClient9getFormatEv");

#define SLOTS 4
#define MAX_LAYERS 32
#define MAX_AUDIO 2

typedef struct {
	void *bits;
	ARect damage;
} slot;

typedef struct {
	int used;
	int orphan;
	uint32_t id;
	void *sc;
	void *surface;
	ANativeWindow *win;
	int w, h;
	uint8_t *shared;
	size_t size;
	slot slots[SLOTS];
	int is_video;
	video vid;
	int is_gl;
	glview gl;
	int is_ui;
	uiview ui;
} layer;

typedef struct {
	int used;
	uint32_t id;
	audio dec;
} audio_slot;

static void *client;
static layer layers[MAX_LAYERS];
static audio_slot audios[MAX_AUDIO];

static int composer(void) {
	void *ps = NULL;
	ps_self(&ps);
	if (ps) ps_start_pool(ps);
	client = calloc(1, 512);
	sf_client_ctor(client);
	rb_incstrong(client, client);
	return sf_client_check(client);
}

static ANativeWindow *window_of(void *surface) {
	for (int off = 0; off < 64; off += 4)
		if (*(uint32_t *)((char *)surface + off) == WINDOW_MAGIC)
			return (ANativeWindow *)((char *)surface + off);
	return NULL;
}

static int last_w, last_h, last_rot;

static layer *find(uint32_t id) {
	for (int i = 0; i < MAX_LAYERS; i++)
		if (layers[i].used && !layers[i].orphan && layers[i].id == id) return &layers[i];
	return NULL;
}

static void drop(layer *l) {
	if (l->is_video) video_close(&l->vid);
	if (l->is_gl) gl_close(&l->gl);
	if (l->is_ui) ui_close(&l->ui);
	if (l->sc) {
		sf_open();
		sc_hide(l->sc);
		sf_close(1);
		sc_clear(l->sc);
		rb_decstrong(l->sc, &l->sc);
	}
	if (l->surface) rb_decstrong(l->surface, &l->surface);
	if (l->shared) munmap(l->shared, l->size);
	memset(l, 0, sizeof *l);
}

static void drop_orphans(void) {
	for (int i = 0; i < MAX_LAYERS; i++)
		if (layers[i].used && layers[i].orphan) drop(&layers[i]);
}

static void orphan_all(void) {
	for (int i = 0; i < MAX_LAYERS; i++)
		if (layers[i].used) layers[i].orphan = 1;
}

static void drop_protected(AMediaCrypto *crypto) {
	for (int i = 0; i < MAX_LAYERS; i++)
		if (layers[i].used && layers[i].is_video && layers[i].vid.crypto && (!crypto || layers[i].vid.crypto == crypto)) drop(&layers[i]);
}

static audio_slot *find_audio(uint32_t id) {
	for (int i = 0; i < MAX_AUDIO; i++)
		if (audios[i].used && audios[i].id == id) return &audios[i];
	return NULL;
}

static void close_audio(audio_slot *a) {
	audio_close(&a->dec);
	memset(a, 0, sizeof *a);
}

static void release_protected(void) {
	for (int i = 0; i < MAX_AUDIO; i++)
		if (audios[i].used) close_audio(&audios[i]);
	drop_protected(NULL);
	drm_close_all();
}

static void grow(ARect *a, const ARect *b) {
	if (a->right <= a->left || a->bottom <= a->top) { *a = *b; return; }
	if (b->left < a->left) a->left = b->left;
	if (b->top < a->top) a->top = b->top;
	if (b->right > a->right) a->right = b->right;
	if (b->bottom > a->bottom) a->bottom = b->bottom;
}

static int send_msg(int sock, const void *buf, size_t len, int fd) {
	struct iovec iov = {(void *)buf, len};
	char ctl[CMSG_SPACE(sizeof(int))];
	struct msghdr msg = {0};
	msg.msg_iov = &iov;
	msg.msg_iovlen = 1;
	if (fd >= 0) {
		memset(ctl, 0, sizeof ctl);
		msg.msg_control = ctl;
		msg.msg_controllen = sizeof ctl;
		struct cmsghdr *c = CMSG_FIRSTHDR(&msg);
		c->cmsg_level = SOL_SOCKET;
		c->cmsg_type = SCM_RIGHTS;
		c->cmsg_len = CMSG_LEN(sizeof(int));
		memcpy(CMSG_DATA(c), &fd, sizeof(int));
	}
	return sendmsg(sock, &msg, MSG_NOSIGNAL) == (ssize_t)len ? 0 : -1;
}

static int reply(int sock, uint32_t op, const uint32_t *words, uint32_t n, int fd) {
	uint32_t buf[2 + 8];
	buf[0] = op;
	buf[1] = n * 4;
	memcpy(buf + 2, words, n * 4);
	return send_msg(sock, buf, (2 + n) * 4, fd);
}

static int reply_data(int sock, uint32_t op, const uint32_t *words, uint32_t n, const void *data, size_t len) {
	static const uint8_t zeros[3];
	uint32_t head[2 + 8];
	size_t pad;
	size_t hl = wire_head(head, op, words, n, len, &pad);
	struct iovec iov[3] = {{head, hl}, {(void *)data, len}, {(void *)zeros, pad}};
	struct msghdr msg = {0};
	msg.msg_iov = iov;
	msg.msg_iovlen = 3;
	return sendmsg(sock, &msg, MSG_NOSIGNAL) == (ssize_t)(hl + len + pad) ? 0 : -1;
}

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

static layer *free_slot(void) {
	for (int i = 0; i < MAX_LAYERS; i++)
		if (!layers[i].used) return &layers[i];
	return NULL;
}

static int surface_for(layer *l, uint32_t id, uint32_t w, uint32_t h, int32_t x, int32_t y, int32_t z, uint32_t flags) {
	void *name = calloc(1, 16);
	s8_ctor(name, "lanovo");
	uint32_t sfflags = (flags & FLAG_OPAQUE ? SF_OPAQUE : 0) | (flags & FLAG_SECURE ? SF_SECURE : 0);
	void *sc = NULL;
	sf_create(&sc, client, name, w, h, PIXEL_FORMAT_RGBA_8888, sfflags, NULL, 0, 0);
	if (!sc) return ERR_SURFACE;

	sf_open();
	sc_layer(sc, z);
	sc_position(sc, (float)x, (float)y);
	sc_show(sc);
	sf_close(1);

	void *surface = NULL;
	sc_surface(&surface, sc);
	ANativeWindow *win = surface ? window_of(surface) : NULL;
	if (!win) {
		sc_clear(sc);
		return ERR_SURFACE;
	}
	memset(l, 0, sizeof *l);
	l->used = 1;
	l->id = id;
	l->sc = sc;
	l->surface = surface;
	l->win = win;
	l->w = w;
	l->h = h;
	return OK;
}

static int create(int conn, const uint32_t *a) {
	uint32_t id = a[0], w = a[3], h = a[4], flags = a[6];
	int32_t x = (int32_t)a[1], y = (int32_t)a[2], z = (int32_t)a[5];
	if (find(id) || w == 0 || h == 0 || w > 4096 || h > 4096) return reply(conn, OP_CREATE, (uint32_t[]){id, ERR_ARGS}, 2, -1);

	layer *l = free_slot();
	if (!l) return reply(conn, OP_CREATE, (uint32_t[]){id, ERR_FULL}, 2, -1);
	int st = surface_for(l, id, w, h, x, y, z, flags);
	if (st != OK) return reply(conn, OP_CREATE, (uint32_t[]){id, (uint32_t)st}, 2, -1);
	void *sc = l->sc;
	ANativeWindow_setBuffersGeometry(l->win, w, h, WINDOW_FORMAT_RGBA_8888);

	size_t size = (size_t)w * h * 4;
	int mfd = syscall(__NR_memfd_create, "lanovo-layer", 0);
	uint8_t *shared = MAP_FAILED;
	if (mfd >= 0 && ftruncate(mfd, size) == 0)
		shared = mmap(NULL, size, PROT_READ, MAP_SHARED, mfd, 0);
	if (shared == MAP_FAILED) {
		if (mfd >= 0) close(mfd);
		sc_clear(sc);
		memset(l, 0, sizeof *l);
		return reply(conn, OP_CREATE, (uint32_t[]){id, ERR_MEMORY}, 2, -1);
	}

	l->shared = shared;
	l->size = size;
	int err = reply(conn, OP_CREATE, (uint32_t[]){id, OK, w * 4}, 3, mfd);
	close(mfd);
	return err;
}

static int post(layer *l, const uint32_t *rects, uint32_t n) {
	for (uint32_t i = 0; i < n; i++) {
		const uint32_t *r = rects + i * 4;
		ARect c = {(int32_t)r[0], (int32_t)r[1], (int32_t)(r[0] + r[2]), (int32_t)(r[1] + r[3])};
		for (int s = 0; s < SLOTS; s++)
			if (l->slots[s].bits) grow(&l->slots[s].damage, &c);
	}

	ANativeWindow_Buffer buf;
	int32_t st = ANativeWindow_lock(l->win, &buf, NULL);
	if (st != 0) return ERR_SURFACE;

	ARect dirty = {0, 0, l->w, l->h};
	int found = -1, empty = -1;
	for (int s = 0; s < SLOTS; s++) {
		if (l->slots[s].bits == buf.bits) found = s;
		else if (!l->slots[s].bits && empty < 0) empty = s;
	}
	if (found >= 0) {
		dirty = l->slots[found].damage;
		l->slots[found].damage = (ARect){0, 0, 0, 0};
	} else if (empty >= 0) {
		l->slots[empty].bits = buf.bits;
	}

	int x0 = dirty.left < 0 ? 0 : dirty.left, y0 = dirty.top < 0 ? 0 : dirty.top;
	int x1 = dirty.right > l->w ? l->w : dirty.right, y1 = dirty.bottom > l->h ? l->h : dirty.bottom;
	size_t stride = (size_t)l->w * 4;
	size_t row = x1 > x0 ? (size_t)(x1 - x0) * 4 : 0;
	for (int y = y0; row && y < y1; y++)
		memcpy((uint8_t *)buf.bits + ((size_t)y * buf.stride + x0) * 4, l->shared + (size_t)y * stride + (size_t)x0 * 4, row);

	return ANativeWindow_unlockAndPost(l->win) == 0 ? OK : ERR_SURFACE;
}

static int frame(int conn, const uint32_t *a, uint32_t len, int *fresh) {
	uint32_t id = a[0], seq = a[1], n = a[2];
	if (!wire_items(len, 12, n, 16)) return -1;
	layer *l = find(id);
	int st = l ? post(l, a + 3, n) : ERR_ARGS;
	if (st == OK && *fresh) {
		drop_orphans();
		*fresh = 0;
	}
	return reply(conn, OP_FRAME, (uint32_t[]){id, seq, (uint32_t)st}, 3, -1);
}

static int scene(int conn, const uint32_t *a, uint32_t len) {
	uint32_t n = a[0];
	if (!wire_items(len, 4, n, 32)) return -1;
	int st = OK;
	sf_open();
	for (uint32_t i = 0; i < n; i++) {
		const uint32_t *e = a + 1 + i * 8;
		layer *l = find(e[0]);
		if (!l) { st = ERR_ARGS; continue; }
		sc_position(l->sc, (float)(int32_t)e[1], (float)(int32_t)e[2]);
		if (e[3] && e[4]) sc_size(l->sc, e[3], e[4]);
		sc_layer(l->sc, (int32_t)e[5]);
		sc_alpha(l->sc, e[7] / 1000.0f);
		if (e[6]) sc_show(l->sc);
		else sc_hide(l->sc);
	}
	sf_close(1);
	return reply(conn, OP_SCENE, (uint32_t[]){(uint32_t)st}, 1, -1);
}

static float as_float(uint32_t bits) {
	float f;
	memcpy(&f, &bits, sizeof f);
	return f;
}

static int video_new(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], codec = a[1], w = a[2], h = a[3], session = a[5], n = a[6];
	int32_t z = (int32_t)a[4];
	if (!wire_bytes(len, 28, n)) return -1;
	char decoder[128];
	if (n >= sizeof decoder) return reply(conn, OP_VIDEO_OPEN, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	memcpy(decoder, a + 7, n);
	decoder[n] = 0;
	if (find(id) || w == 0 || h == 0 || w > 4096 || h > 4096) return reply(conn, OP_VIDEO_OPEN, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	AMediaCrypto *crypto = NULL;
	if (session && !(crypto = drm_crypto(session))) return reply(conn, OP_VIDEO_OPEN, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	layer *l = free_slot();
	if (!l) return reply(conn, OP_VIDEO_OPEN, (uint32_t[]){id, ERR_FULL}, 2, -1);
	int st = surface_for(l, id, w, h, 0, 0, z, crypto ? FLAG_SECURE : 0);
	if (st == OK) {
		st = video_open(&l->vid, l->win, codec, w, h, crypto, decoder);
		if (st == OK) l->is_video = 1;
		else drop(l);
	}
	return reply(conn, OP_VIDEO_OPEN, (uint32_t[]){id, (uint32_t)st}, 2, -1);
}

static int video_feed(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], flags = a[3], n = a[4];
	if (!wire_bytes(len, 20, n)) return -1;
	int64_t pts = (int64_t)((uint64_t)a[2] << 32 | a[1]);
	layer *l = find(id);
	int st = l && l->is_video ? video_sample(&l->vid, pts, flags, NULL, (const uint8_t *)(a + 5), n) : ERR_ARGS;
	uint32_t shown = l ? l->vid.shown : 0, dropped = l ? l->vid.dropped : 0;
	return reply(conn, OP_VIDEO_SAMPLE, (uint32_t[]){id, (uint32_t)st, shown, dropped, l ? (uint32_t)l->vid.ended : 0}, 5, -1);
}

static int video_crypt(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], flags = a[3], n = a[4];
	int64_t pts = (int64_t)((uint64_t)a[2] << 32 | a[1]);
	drm_sample s;
	const uint32_t *data = wire_crypt(a + 5, len / 4 - 5, &s);
	if (!data || !wire_bytes(len, (uint64_t)(data - a) * 4, n)) return -1;
	layer *l = find(id);
	int st = l && l->is_video ? video_sample(&l->vid, pts, flags, &s, (const uint8_t *)data, n) : ERR_ARGS;
	uint32_t shown = l ? l->vid.shown : 0, dropped = l ? l->vid.dropped : 0;
	return reply(conn, OP_VIDEO_SAMPLE, (uint32_t[]){id, (uint32_t)st, shown, dropped, l ? (uint32_t)l->vid.ended : 0}, 5, -1);
}

static int drm_new(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], flags = a[1], n = a[2];
	if (!wire_bytes(len, 28, n)) return -1;
	int st = drm_open(id, (const uint8_t *)(a + 3), (int)(flags & 1), (const uint8_t *)(a + 7), n);
	return reply(conn, OP_DRM_OPEN, (uint32_t[]){id, (uint32_t)st}, 2, -1);
}

static int drm_prov(int conn, const uint32_t *a) {
	const uint8_t *req = NULL;
	size_t req_len = 0;
	const char *url = NULL;
	int st = drm_provision((const uint8_t *)a, &req, &req_len, &url);
	size_t url_len = st == OK && url ? strlen(url) : 0;
	uint8_t *body = malloc(req_len + url_len + 1);
	if (!body) return reply(conn, OP_DRM_PROVISION, (uint32_t[]){ERR_MEMORY, 0, 0}, 3, -1);
	if (req_len) memcpy(body, req, req_len);
	if (url_len) memcpy(body + req_len, url, url_len);
	int r = reply_data(conn, OP_DRM_PROVISION, (uint32_t[]){(uint32_t)st, (uint32_t)req_len, (uint32_t)(req_len + url_len)}, 3, body, req_len + url_len);
	free(body);
	return r;
}

static int drm_cert(int conn, const uint32_t *a, uint32_t len) {
	uint32_t n = a[0];
	if (!wire_bytes(len, 4, n)) return -1;
	int st = drm_provisioned((const uint8_t *)(a + 1), n);
	return reply(conn, OP_DRM_PROVISIONED, (uint32_t[]){(uint32_t)st}, 1, -1);
}

static int drm_ask(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], n = a[1];
	if (!wire_bytes(len, 8, n)) return -1;
	const uint8_t *req = NULL;
	size_t req_len = 0;
	int st = drm_request(id, (const uint8_t *)(a + 2), n, &req, &req_len);
	if (st != OK) req_len = 0;
	return reply_data(conn, OP_DRM_REQUEST, (uint32_t[]){id, (uint32_t)st, (uint32_t)req_len}, 3, req, req_len);
}

static int drm_answer(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], n = a[1];
	if (!wire_bytes(len, 8, n)) return -1;
	int st = drm_provide(id, (const uint8_t *)(a + 2), n);
	return reply(conn, OP_DRM_PROVIDE, (uint32_t[]){id, (uint32_t)st}, 2, -1);
}

static void drm_end(uint32_t id) {
	AMediaCrypto *c = drm_existing(id);
	if (c) {
		for (int i = 0; i < MAX_AUDIO; i++)
			if (audios[i].used && audios[i].dec.crypto == c) close_audio(&audios[i]);
		drop_protected(c);
	}
	drm_close(id);
}

static int audio_new(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], session = a[1], rate = a[2], channels = a[3], n = a[4];
	if (!wire_bytes(len, 20, n)) return -1;
	int st = ERR_ARGS;
	AMediaCrypto *crypto = session ? drm_crypto(session) : NULL;
	audio_slot *slot = NULL;
	for (int i = 0; i < MAX_AUDIO && !slot; i++)
		if (!audios[i].used) slot = &audios[i];
	if (find_audio(id) || (session && !crypto)) st = ERR_ARGS;
	else if (!slot) st = ERR_FULL;
	else if ((st = audio_open(&slot->dec, crypto, rate, channels, (const uint8_t *)(a + 5), n)) == OK) {
		slot->used = 1;
		slot->id = id;
	}
	return reply(conn, OP_AUDIO_OPEN, (uint32_t[]){id, (uint32_t)st}, 2, -1);
}

static int audio_feed(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], flags = a[3], n = a[4];
	int64_t pts = (int64_t)((uint64_t)a[2] << 32 | a[1]);
	drm_sample s;
	const uint32_t *data = wire_crypt(a + 5, len / 4 - 5, &s);
	if (!data || !wire_bytes(len, (uint64_t)(data - a) * 4, n)) return -1;
	audio_slot *slot = find_audio(id);
	size_t pcm = 0;
	int st = slot ? audio_sample(&slot->dec, pts, flags, &s, (const uint8_t *)data, n, &pcm) : ERR_ARGS;
	uint32_t rate = slot ? (uint32_t)slot->dec.rate : 0, ch = slot ? (uint32_t)slot->dec.channels : 0;
	return reply_data(conn, OP_AUDIO_SAMPLE, (uint32_t[]){id, (uint32_t)st, rate, ch, (uint32_t)pcm}, 5, slot ? slot->dec.pcm : NULL, pcm);
}

static void video_time(const uint32_t *a) {
	layer *l = find(a[0]);
	if (l && l->is_video) video_clock(&l->vid, (int64_t)((uint64_t)a[2] << 32 | a[1]), (int)a[3]);
}

static int video_place(int conn, const uint32_t *a) {
	layer *l = find(a[0]);
	if (!l) return reply(conn, OP_VIDEO_PLACE, (uint32_t[]){ERR_ARGS}, 1, -1);
	sf_open();
	sc_position(l->sc, as_float(a[1]), as_float(a[2]));
	sc_matrix(l->sc, as_float(a[3]), as_float(a[4]), as_float(a[5]), as_float(a[6]));
	sc_layer(l->sc, (int32_t)a[7]);
	if (a[8]) sc_show(l->sc);
	else sc_hide(l->sc);
	sf_close(1);
	return reply(conn, OP_VIDEO_PLACE, (uint32_t[]){OK}, 1, -1);
}

static int gl_new(int conn, const uint32_t *a) {
	uint32_t id = a[0], w = a[1], h = a[2];
	int32_t z = (int32_t)a[3];
	if (find(id) || w == 0 || h == 0 || w > 4096 || h > 4096) return reply(conn, OP_GL_OPEN, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	layer *l = free_slot();
	if (!l) return reply(conn, OP_GL_OPEN, (uint32_t[]){id, ERR_FULL}, 2, -1);
	int st = surface_for(l, id, w, h, 0, 0, z, 0);
	if (st == OK) {
		st = gl_open(&l->gl, l->win, w, h);
		if (st == OK) l->is_gl = 1;
		else drop(l);
	}
	return reply(conn, OP_GL_OPEN, (uint32_t[]){id, (uint32_t)st}, 2, -1);
}

static int ui_new(int conn, const uint32_t *a) {
	uint32_t id = a[0], w = a[1], h = a[2];
	int32_t z = (int32_t)a[3];
	if (find(id) || w == 0 || h == 0 || w > 4096 || h > 4096) return reply(conn, OP_UI_OPEN, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	layer *l = free_slot();
	if (!l) return reply(conn, OP_UI_OPEN, (uint32_t[]){id, ERR_FULL}, 2, -1);
	int st = surface_for(l, id, w, h, 0, 0, z, 0);
	if (st == OK) {
		last_w = (int)w;
		last_h = (int)h;
		st = ui_open(&l->ui, l->win, (int)w, (int)h);
		if (st == OK) l->is_ui = 1;
		else drop(l);
	}
	return reply(conn, OP_UI_OPEN, (uint32_t[]){id, (uint32_t)st}, 2, -1);
}

static int ui_source(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], slot = a[1], vslen = a[2], fslen = a[3];
	if (!wire_bytes(len, 16, (uint64_t)vslen + fslen)) return -1;
	layer *l = find(id);
	int st = ERR_ARGS;
	if (l && l->is_ui && slot < UI_PROGRAMS) {
		const char *src = (const char *)(a + 4);
		ui_program(&l->ui, (int)slot, src, (int)vslen, src + vslen, (int)fslen);
		st = OK;
	}
	return reply(conn, OP_UI_PROGRAM, (uint32_t[]){(uint32_t)st}, 1, -1);
}

static int ui_tex(const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], tex = a[1], w = a[2], h = a[3], x = a[4], y = a[5], rw = a[6], rh = a[7];
	if (w > 4096 || h > 4096 || rw > w || rh > h || x + rw > w || y + rh > h) return -1;
	if ((uint64_t)rw * rh * 4 + 32 != len) return -1;
	layer *l = find(id);
	if (l && l->is_ui) ui_texture(&l->ui, tex, (int)w, (int)h, (int)x, (int)y, (int)rw, (int)rh, (const uint8_t *)(a + 8));
	return 0;
}

static int ui_draw(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], rot = a[5], nquads = a[6], nruns = a[7];
	last_rot = (int)(rot & 3);
	if ((uint64_t)nquads * UI_QUAD_FLOATS * 4 + (uint64_t)nruns * UI_RUN_WORDS * 4 + 32 != len) return -1;
	layer *l = find(id);
	if (!l || !l->is_ui) return reply(conn, OP_UI_FRAME, (uint32_t[]){ERR_ARGS, 0, 0, 0}, 4, -1);
	float clear[4];
	memcpy(clear, a + 1, sizeof clear);
	const float *quads = (const float *)(a + 8);
	const uint32_t *runs = a + 8 + nquads * UI_QUAD_FLOATS;
	uint32_t frame = ui_frame(&l->ui, (int)(rot & 3), clear, quads, (int)nquads, runs, (int)nruns);
	uint32_t submit = 0, finish = 0;
	ui_wait(&l->ui, frame, &submit, &finish);
	return reply(conn, OP_UI_FRAME, (uint32_t[]){OK, frame, submit, finish}, 4, -1);
}

static int ui_readback(int conn, const uint32_t *a) {
	uint32_t id = a[0];
	layer *l = find(id);
	if (!l || !l->is_ui) return reply(conn, OP_UI_READ, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	size_t size = (size_t)l->ui.w * l->ui.h * 4;
	int mfd = syscall(__NR_memfd_create, "lanovo-read", 0);
	uint8_t *pix = MAP_FAILED;
	if (mfd >= 0 && ftruncate(mfd, size) == 0) pix = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, mfd, 0);
	if (pix == MAP_FAILED) {
		if (mfd >= 0) close(mfd);
		return reply(conn, OP_UI_READ, (uint32_t[]){id, ERR_MEMORY}, 2, -1);
	}
	int st = ui_read(&l->ui, pix);
	munmap(pix, size);
	int err = reply(conn, OP_UI_READ, (uint32_t[]){id, (uint32_t)st, (uint32_t)l->ui.w, (uint32_t)l->ui.h}, 4, st == OK ? mfd : -1);
	close(mfd);
	return err;
}

static int gl_offscreen(int conn, const uint32_t *a) {
	uint32_t id = a[0], w = a[1], h = a[2];
	if (find(id) || w == 0 || h == 0 || w > 4096 || h > 4096) return reply(conn, OP_GL_OFFSCREEN, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	layer *l = free_slot();
	if (!l) return reply(conn, OP_GL_OFFSCREEN, (uint32_t[]){id, ERR_FULL}, 2, -1);
	memset(l, 0, sizeof *l);
	l->used = 1;
	l->id = id;
	l->w = (int)w;
	l->h = (int)h;
	int st = gl_open(&l->gl, NULL, (int)w, (int)h);
	if (st == OK) l->is_gl = 1;
	else memset(l, 0, sizeof *l);
	return reply(conn, OP_GL_OFFSCREEN, (uint32_t[]){id, (uint32_t)st}, 2, -1);
}

static int gl_readback(int conn, const uint32_t *a) {
	uint32_t id = a[0];
	layer *l = find(id);
	if (!l || !l->is_gl) return reply(conn, OP_GL_READ, (uint32_t[]){id, ERR_ARGS}, 2, -1);
	size_t size = (size_t)l->gl.w * l->gl.h * 4;
	int mfd = syscall(__NR_memfd_create, "lanovo-read", 0);
	uint8_t *pix = MAP_FAILED;
	if (mfd >= 0 && ftruncate(mfd, size) == 0) pix = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, mfd, 0);
	if (pix == MAP_FAILED) {
		if (mfd >= 0) close(mfd);
		return reply(conn, OP_GL_READ, (uint32_t[]){id, ERR_MEMORY}, 2, -1);
	}
	int st = gl_read(&l->gl, pix);
	munmap(pix, size);
	int err = reply(conn, OP_GL_READ, (uint32_t[]){id, (uint32_t)st, (uint32_t)l->gl.w, (uint32_t)l->gl.h}, 4, st == OK ? mfd : -1);
	close(mfd);
	return err;
}

#define PIXEL_RGBA_8888 1
#define PIXEL_RGBX_8888 2
#define PIXEL_BGRA_8888 5

static int screen_read(int conn) {
	void *display = NULL;
	sf_display(&display, 0);
	if (!display) return reply(conn, OP_SCREEN_READ, (uint32_t[]){ERR_SURFACE}, 1, -1);

	uint64_t client[64] = {0};
	ss_ctor(client);
	int got = ss_update(client, &display, 0, 0, 0, 0, 0);
	const uint8_t *src = got == 0 ? ss_pixels(client) : NULL;
	uint32_t w = src ? ss_width(client) : 0, h = src ? ss_height(client) : 0, stride = src ? ss_stride(client) : 0;
	int32_t format = src ? ss_format(client) : 0;
	if (!src || !w || !h || (format != PIXEL_RGBA_8888 && format != PIXEL_RGBX_8888 && format != PIXEL_BGRA_8888)) {
		logw("screen capture: status %d format %d %ux%u", got, format, w, h);
		ss_dtor(client);
		return reply(conn, OP_SCREEN_READ, (uint32_t[]){ERR_SURFACE}, 1, -1);
	}

	size_t size = (size_t)w * h * 4;
	int mfd = syscall(__NR_memfd_create, "lanovo-screen", 0);
	uint8_t *pix = MAP_FAILED;
	if (mfd >= 0 && ftruncate(mfd, size) == 0) pix = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, mfd, 0);
	if (pix == MAP_FAILED) {
		if (mfd >= 0) close(mfd);
		ss_dtor(client);
		return reply(conn, OP_SCREEN_READ, (uint32_t[]){ERR_MEMORY}, 1, -1);
	}
	for (uint32_t y = 0; y < h; y++) {
		const uint8_t *in = src + (size_t)y * stride * 4;
		uint8_t *out = pix + (size_t)y * w * 4;
		for (uint32_t x = 0; x < w; x++, in += 4, out += 4) {
			if (format == PIXEL_BGRA_8888) {
				out[0] = in[2], out[1] = in[1], out[2] = in[0];
			} else {
				out[0] = in[0], out[1] = in[1], out[2] = in[2];
			}
			out[3] = 0xff;
		}
	}
	munmap(pix, size);
	ss_dtor(client);
	int err = reply(conn, OP_SCREEN_READ, (uint32_t[]){OK, w, h}, 3, mfd);
	close(mfd);
	return err;
}

static int gl_upload(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], unit = a[1], w = a[2], h = a[3];
	if ((uint64_t)w * h * 4 + 16 != len) return -1;
	layer *l = find(id);
	int st = ERR_ARGS;
	if (l && l->is_gl && unit < GL_UNITS) {
		gl_texture(&l->gl, (int)unit, (int)w, (int)h, (const uint8_t *)(a + 4));
		st = OK;
	}
	return reply(conn, OP_GL_TEXTURE, (uint32_t[]){(uint32_t)st}, 1, -1);
}

static int gl_source(int conn, const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], flags = a[1], n = a[2];
	if (!wire_bytes(len, 12, n)) return -1;
	layer *l = find(id);
	int st = l && l->is_gl ? gl_program(&l->gl, (const char *)(a + 3), (int)n, (int)flags) : ERR_ARGS;
	return reply(conn, OP_GL_PROGRAM, (uint32_t[]){(uint32_t)st}, 1, -1);
}

static void gl_set(const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], n = a[1];
	if (!wire_items(len, 20, n, 4)) return;
	layer *l = find(id);
	gl_glow glow;
	memcpy(&glow.amount, a + 2, 4);
	memcpy(&glow.radius, a + 3, 4);
	glow.passes = (int)a[4];
	if (l && l->is_gl) gl_values(&l->gl, (const float *)(a + 5), (int)n, glow);
}

static void gl_dots(const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], n = a[1];
	if (n > 65536 || 8 + (uint64_t)n * GL_POINT_FLOATS * 4 != len) return;
	layer *l = find(id);
	if (l && l->is_gl) gl_points(&l->gl, (const float *)(a + 2), (int)n);
}

static void gl_segs(const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], n = a[1];
	if (n > 65536 || 8 + (uint64_t)n * GL_LINE_FLOATS * 4 != len) return;
	layer *l = find(id);
	if (l && l->is_gl) gl_lines(&l->gl, (const float *)(a + 2), (int)n);
}

static void gl_fills(const uint32_t *a, uint32_t len) {
	uint32_t id = a[0], n = a[1];
	if (n > 65536 || 8 + (uint64_t)n * GL_QUAD_FLOATS * 4 != len) return;
	layer *l = find(id);
	if (l && l->is_gl) gl_quads(&l->gl, (const float *)(a + 2), (int)n);
}

static void serve(int conn) {
	uint32_t hdr[2];
	if (read_full(conn, hdr, sizeof hdr) < 0 || hdr[0] != OP_HELLO || hdr[1] != 8) return;
	uint32_t hello[2];
	if (read_full(conn, hello, sizeof hello) < 0 || hello[0] != MAGIC) return;
	if (hello[1] != VERSION) {
		reply(conn, OP_HELLO, (uint32_t[]){VERSION}, 1, -1);
		logw("client speaks version %u, this is %u", hello[1], VERSION);
		return;
	}
	if (reply(conn, OP_HELLO, (uint32_t[]){VERSION}, 1, -1) < 0) return;

	int fresh = 1;
	uint32_t *buf = NULL;
	uint32_t cap = 0;
	for (;;) {
		if (read_full(conn, hdr, sizeof hdr) < 0) break;
		if (hdr[1] > 64 << 20) break;
		if (hdr[1] > cap || (cap > 1 << 20 && hdr[1] <= 1 << 20)) {
			cap = hdr[1] > 1 << 20 ? hdr[1] : 1 << 20;
			free(buf);
			buf = malloc(cap);
			if (!buf) break;
		}
		if (hdr[1] && read_full(conn, buf, hdr[1]) < 0) break;
		int err = 0;
		switch (hdr[0]) {
		case OP_CREATE:
			err = hdr[1] == 28 ? create(conn, buf) : -1;
			break;
		case OP_FRAME:
			err = hdr[1] >= 12 ? frame(conn, buf, hdr[1], &fresh) : -1;
			break;
		case OP_SCENE:
			err = hdr[1] >= 4 ? scene(conn, buf, hdr[1]) : -1;
			break;
		case OP_DESTROY: {
			layer *l = hdr[1] == 4 ? find(buf[0]) : NULL;
			if (l) drop(l);
			break;
		}
		case OP_VIDEO_CLOSE: {
			layer *l = hdr[1] == 8 ? find(buf[0]) : NULL;
			if (l && !buf[1]) drop(l);
			break;
		}
		case OP_VIDEO_FLUSH: {
			layer *l = hdr[1] == 4 ? find(buf[0]) : NULL;
			int st = l && l->is_video ? video_flush(&l->vid) : ERR_ARGS;
			err = reply(conn, OP_VIDEO_FLUSH, (uint32_t[]){(uint32_t)st}, 1, -1);
			break;
		}
		case OP_GL_OPEN:
			err = hdr[1] == 16 ? gl_new(conn, buf) : -1;
			break;
		case OP_GL_TEXTURE:
			err = hdr[1] >= 16 ? gl_upload(conn, buf, hdr[1]) : -1;
			break;
		case OP_GL_PROGRAM:
			err = hdr[1] >= 12 ? gl_source(conn, buf, hdr[1]) : -1;
			break;
		case OP_GL_LINES:
			if (hdr[1] >= 8) gl_segs(buf, hdr[1]);
			else err = -1;
			break;
		case OP_GL_OFFSCREEN:
			err = hdr[1] == 12 ? gl_offscreen(conn, buf) : -1;
			break;
		case OP_GL_READ:
			err = hdr[1] == 4 ? gl_readback(conn, buf) : -1;
			break;
		case OP_UI_OPEN:
			err = hdr[1] == 16 ? ui_new(conn, buf) : -1;
			break;
		case OP_UI_PROGRAM:
			err = hdr[1] >= 16 ? ui_source(conn, buf, hdr[1]) : -1;
			break;
		case OP_UI_TEXTURE:
			err = hdr[1] >= 32 ? ui_tex(buf, hdr[1]) : -1;
			break;
		case OP_UI_FRAME:
			err = hdr[1] >= 32 ? ui_draw(conn, buf, hdr[1]) : -1;
			break;
		case OP_UI_READ:
			err = hdr[1] == 4 ? ui_readback(conn, buf) : -1;
			break;
		case OP_SCREEN_READ:
			err = hdr[1] == 0 ? screen_read(conn) : -1;
			break;
		case OP_GL_QUADS:
			if (hdr[1] >= 8) gl_fills(buf, hdr[1]);
			else err = -1;
			break;
		case OP_GL_POINTS:
			if (hdr[1] >= 8) gl_dots(buf, hdr[1]);
			else err = -1;
			break;
		case OP_GL_VALUES:
			if (hdr[1] >= 20) gl_set(buf, hdr[1]);
			else err = -1;
			break;
		case OP_VIDEO_OPEN:
			err = hdr[1] >= 28 ? video_new(conn, buf, hdr[1]) : -1;
			break;
		case OP_VIDEO_CRYPT:
			err = hdr[1] >= 20 + CRYPT_WORDS * 4 ? video_crypt(conn, buf, hdr[1]) : -1;
			break;
		case OP_DRM_OPEN:
			err = hdr[1] >= 28 ? drm_new(conn, buf, hdr[1]) : -1;
			break;
		case OP_DRM_PROVISION:
			err = hdr[1] == 16 ? drm_prov(conn, buf) : -1;
			break;
		case OP_DRM_PROVISIONED:
			err = hdr[1] >= 4 ? drm_cert(conn, buf, hdr[1]) : -1;
			break;
		case OP_DRM_REQUEST:
			err = hdr[1] >= 8 ? drm_ask(conn, buf, hdr[1]) : -1;
			break;
		case OP_DRM_PROVIDE:
			err = hdr[1] >= 8 ? drm_answer(conn, buf, hdr[1]) : -1;
			break;
		case OP_DRM_CLOSE:
			if (hdr[1] == 4) drm_end(buf[0]);
			else err = -1;
			break;
		case OP_AUDIO_OPEN:
			err = hdr[1] >= 20 ? audio_new(conn, buf, hdr[1]) : -1;
			break;
		case OP_AUDIO_SAMPLE:
			err = hdr[1] >= 20 + CRYPT_WORDS * 4 ? audio_feed(conn, buf, hdr[1]) : -1;
			break;
		case OP_AUDIO_CLOSE: {
			audio_slot *s = hdr[1] == 4 ? find_audio(buf[0]) : NULL;
			if (s) close_audio(s);
			else if (hdr[1] != 4) err = -1;
			break;
		}
		case OP_VIDEO_SAMPLE:
			err = hdr[1] >= 20 ? video_feed(conn, buf, hdr[1]) : -1;
			break;
		case OP_VIDEO_CLOCK:
			if (hdr[1] == 16) video_time(buf);
			else err = -1;
			break;
		case OP_VIDEO_PLACE:
			err = hdr[1] == 36 ? video_place(conn, buf) : -1;
			break;
		default:
			err = -1;
		}
		if (err < 0) break;
	}
	free(buf);
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

#define WAIT_ID 0xfffffff0u
#define WAIT_Z 1000

static layer *wait_layer;
static pthread_t wait_th;
static volatile int waiting;

static const char wait_vs[] =
	"layout(location=0) in vec3 p;\n"
	"layout(location=1) in vec2 uv;\n"
	"layout(location=2) in vec4 c;\n"
	"uniform mat4 mvp;\n"
	"out vec2 v;\n"
	"out vec4 col;\n"
	"void main() { v = uv; col = c; gl_Position = mvp * vec4(p.xy, 0.0, 1.0); }\n";

static const char wait_fs[] =
	"in vec2 v;\n"
	"in vec4 col;\n"
	"out vec4 o;\n"
	"void main() {\n"
	"	float d = length(v);\n"
	"	float inner = 1.0 - smoothstep(0.6, 0.7, d);\n"
	"	o = vec4(mix(vec3(0.05), col.rgb, inner), (1.0 - smoothstep(0.88, 1.0, d)) * mix(0.85, col.a, inner));\n"
	"}\n";

static void *wait_loop(void *arg) {
	layer *l = arg;
	float vw = last_rot & 1 ? l->h : l->w, vh = last_rot & 1 ? l->w : l->h;
	float r = (vw < vh ? vw : vh) * 0.018f, gap = r * 3.5f, cx = vw / 2, cy = vh / 2;
	float quads[3 * UI_QUAD_FLOATS];
	uint32_t run[UI_RUN_WORDS] = {0};
	run[7] = 3;
	const float clear[4] = {0, 0, 0, 0};
	struct timespec t0, now;
	clock_gettime(CLOCK_MONOTONIC, &t0);
	while (waiting) {
		clock_gettime(CLOCK_MONOTONIC, &now);
		double t = (now.tv_sec - t0.tv_sec) + (now.tv_nsec - t0.tv_nsec) / 1e9;
		for (int i = 0; i < 3; i++) {
			double pulse = sin(2 * M_PI * (t / 1.2 - i / 6.0));
			float a = 0.25f + 0.75f * (float)(pulse > 0 ? pulse : 0);
			float x = cx + (i - 1) * gap;
			float corners[4][4] = {{x - r, cy - r, -1, -1}, {x + r, cy - r, 1, -1}, {x + r, cy + r, 1, 1}, {x - r, cy + r, -1, 1}};
			for (int k = 0; k < 4; k++) {
				float *q = quads + i * UI_QUAD_FLOATS + k * UI_VERTEX_FLOATS;
				q[0] = corners[k][0], q[1] = corners[k][1], q[2] = 0, q[3] = corners[k][2], q[4] = corners[k][3];
				q[5] = 0.85f, q[6] = 0.85f, q[7] = 0.85f, q[8] = a;
			}
		}
		ui_frame(&l->ui, last_rot, clear, quads, 3, run, 1);
		struct timespec tick = {0, 33 * 1000 * 1000};
		nanosleep(&tick, NULL);
	}
	return NULL;
}

static void panel_size(void) {
	if (last_w && last_h) return;
	void *display = NULL;
	sf_display(&display, 0);
	if (!display) return;
	uint64_t shot[64] = {0};
	ss_ctor(shot);
	if (ss_update(shot, &display, 0, 0, 0, 0, 0) == 0) {
		last_w = (int)ss_width(shot);
		last_h = (int)ss_height(shot);
	}
	ss_dtor(shot);
}

static void wait_start(void) {
	panel_size();
	layer *l = last_w && last_h ? free_slot() : NULL;
	if (!l || surface_for(l, WAIT_ID, (uint32_t)last_w, (uint32_t)last_h, 0, 0, WAIT_Z, 0) != OK) return;
	if (ui_open(&l->ui, l->win, last_w, last_h) != OK) {
		drop(l);
		return;
	}
	l->is_ui = 1;
	ui_program(&l->ui, 0, wait_vs, (int)sizeof wait_vs - 1, wait_fs, (int)sizeof wait_fs - 1);
	wait_layer = l;
	waiting = 1;
	if (pthread_create(&wait_th, NULL, wait_loop, l) != 0) {
		waiting = 0;
		drop(l);
		wait_layer = NULL;
	}
}

static void wait_stop(void) {
	if (!wait_layer) return;
	waiting = 0;
	pthread_join(wait_th, NULL);
	drop(wait_layer);
	wait_layer = NULL;
}

int main(void) {
	signal(SIGPIPE, SIG_IGN);
	int lsock = listener();
	if (lsock < 0 || listen(lsock, 2) < 0) {
		logw("socket %s: %s", SOCKET_PATH, strerror(errno));
		return 1;
	}
	int err = composer();
	if (err) {
		logw("composer client: %d", err);
		return 1;
	}
	logi("ready on %s", SOCKET_PATH);
	for (;;) {
		int conn = accept(lsock, NULL, NULL);
		if (conn < 0) continue;
		wait_stop();
		logi("client connected");
		serve(conn);
		close(conn);
		release_protected();
		orphan_all();
		logi("client gone; keeping its layers until the next one draws");
		wait_start();
	}
}
