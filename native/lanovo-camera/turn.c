#include <stdlib.h>
#include <string.h>

#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES2/gl2.h>
#include <GLES2/gl2ext.h>
#include <android/hardware_buffer.h>
#include <android/log.h>
#include <media/NdkImage.h>
#include <media/NdkImageReader.h>

#include "protocol.h"
#include "turn.h"

#define TAG "lanovo-camera"
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)

#define EGL_RECORDABLE 0x3142
#define CACHE 8

typedef EGLClientBuffer (*client_fn)(const struct AHardwareBuffer *);
typedef EGLImageKHR (*image_fn)(EGLDisplay, EGLContext, EGLenum, EGLClientBuffer, const EGLint *);
typedef EGLBoolean (*unimage_fn)(EGLDisplay, EGLImageKHR);
typedef void (*target_fn)(GLenum, GLeglImageOES);
typedef EGLBoolean (*when_fn)(EGLDisplay, EGLSurface, EGLnsecsANDROID);

typedef struct {
	EGLSurface surface;
	int w, h;
	GLfloat quad[16];
} output;

typedef struct {
	AHardwareBuffer *buffer;
	EGLImageKHR image;
	GLuint tex;
} cached;

struct turner {
	EGLDisplay dpy;
	EGLContext ctx;
	EGLConfig config;
	output out[2];
	int outs;
	AImageReader *reader;
	GLuint prog;
	GLint pos, uv;
	cached cache[CACHE];
	int next;
	client_fn client;
	image_fn image;
	unimage_fn unimage;
	target_fn target;
	when_fn when;
};

static const char *vertex =
	"attribute vec2 pos;\n"
	"attribute vec2 uv;\n"
	"varying vec2 at;\n"
	"void main() { at = uv; gl_Position = vec4(pos, 0.0, 1.0); }\n";

static const char *fragment =
	"#extension GL_OES_EGL_image_external : require\n"
	"precision mediump float;\n"
	"uniform samplerExternalOES tex;\n"
	"varying vec2 at;\n"
	"void main() { gl_FragColor = texture2D(tex, at); }\n";

static GLuint shader(GLenum kind, const char *src) {
	GLuint s = glCreateShader(kind);
	glShaderSource(s, 1, &src, NULL);
	glCompileShader(s);
	GLint ok = 0;
	glGetShaderiv(s, GL_COMPILE_STATUS, &ok);
	if (!ok) {
		char why[512];
		glGetShaderInfoLog(s, sizeof why, NULL, why);
		logw("turn: shader: %s", why);
	}
	return s;
}

static void corner(GLfloat *q, int i, float x, float y, int quarters) {
	float u = (x + 1) / 2, v = (1 - y) / 2, s, t;
	if (quarters & TURN_MIRROR) u = 1 - u;
	switch (quarters & 3) {
	case 1: s = v, t = 1 - u; break;
	case 2: s = 1 - u, t = 1 - v; break;
	case 3: s = 1 - v, t = u; break;
	default: s = u, t = v; break;
	}
	q[i * 4] = x;
	q[i * 4 + 1] = y;
	q[i * 4 + 2] = s;
	q[i * 4 + 3] = t;
}

static int surface(turner *t, int i, ANativeWindow *win, int w, int h, int quarters) {
	output *o = &t->out[i];
	o->surface = eglCreateWindowSurface(t->dpy, t->config, win, NULL);
	if (o->surface == EGL_NO_SURFACE) {
		logw("turn: window surface %d: 0x%x", i, eglGetError());
		return -1;
	}
	o->w = w;
	o->h = h;
	corner(o->quad, 0, -1, -1, quarters);
	corner(o->quad, 1, 1, -1, quarters);
	corner(o->quad, 2, -1, 1, quarters);
	corner(o->quad, 3, 1, 1, quarters);
	return 0;
}

turner *turn_open(ANativeWindow *main, int mw, int mh, ANativeWindow *sub, int sw, int sh, int cw, int ch, int quarters,
	ANativeWindow **camera) {
	turner *t = calloc(1, sizeof *t);
	t->dpy = eglGetDisplay(EGL_DEFAULT_DISPLAY);
	if (t->dpy == EGL_NO_DISPLAY || !eglInitialize(t->dpy, NULL, NULL)) goto fail;
	EGLint want[] = {EGL_RED_SIZE, 8, EGL_GREEN_SIZE, 8, EGL_BLUE_SIZE, 8, EGL_RENDERABLE_TYPE, EGL_OPENGL_ES2_BIT,
		EGL_SURFACE_TYPE, EGL_WINDOW_BIT, EGL_RECORDABLE, 1, EGL_NONE};
	EGLint n = 0;
	if (!eglChooseConfig(t->dpy, want, &t->config, 1, &n) || n < 1) goto fail;
	EGLint ctx[] = {EGL_CONTEXT_CLIENT_VERSION, 2, EGL_NONE};
	t->ctx = eglCreateContext(t->dpy, t->config, EGL_NO_CONTEXT, ctx);
	if (t->ctx == EGL_NO_CONTEXT) goto fail;

	t->client = (client_fn)eglGetProcAddress("eglGetNativeClientBufferANDROID");
	t->image = (image_fn)eglGetProcAddress("eglCreateImageKHR");
	t->unimage = (unimage_fn)eglGetProcAddress("eglDestroyImageKHR");
	t->target = (target_fn)eglGetProcAddress("glEGLImageTargetTexture2DOES");
	t->when = (when_fn)eglGetProcAddress("eglPresentationTimeANDROID");
	if (!t->client || !t->image || !t->unimage || !t->target || !t->when) {
		logw("turn: missing EGL extensions");
		goto fail;
	}

	if (surface(t, 0, main, mw, mh, quarters)) goto fail;
	t->outs = 1;
	if (sub) {
		if (surface(t, 1, sub, sw, sh, quarters)) goto fail;
		t->outs = 2;
	}
	if (!eglMakeCurrent(t->dpy, t->out[0].surface, t->out[0].surface, t->ctx)) goto fail;

	t->prog = glCreateProgram();
	glAttachShader(t->prog, shader(GL_VERTEX_SHADER, vertex));
	glAttachShader(t->prog, shader(GL_FRAGMENT_SHADER, fragment));
	glLinkProgram(t->prog);
	t->pos = glGetAttribLocation(t->prog, "pos");
	t->uv = glGetAttribLocation(t->prog, "uv");

	if (AImageReader_newWithUsage(cw, ch, AIMAGE_FORMAT_PRIVATE, AHARDWAREBUFFER_USAGE_GPU_SAMPLED_IMAGE, 6,
			&t->reader) != AMEDIA_OK ||
		AImageReader_getWindow(t->reader, camera) != AMEDIA_OK) {
		logw("turn: image reader %dx%d", cw, ch);
		goto fail;
	}
	return t;
fail:
	logw("turn: setup failed: 0x%x", eglGetError());
	turn_close(t);
	return NULL;
}

static cached *texture(turner *t, AHardwareBuffer *b) {
	for (int i = 0; i < CACHE; i++)
		if (t->cache[i].buffer == b && t->cache[i].image) return &t->cache[i];
	cached *c = &t->cache[t->next];
	t->next = (t->next + 1) % CACHE;
	if (c->image) t->unimage(t->dpy, c->image);
	if (c->tex) glDeleteTextures(1, &c->tex);
	memset(c, 0, sizeof *c);
	EGLint attrs[] = {EGL_IMAGE_PRESERVED_KHR, EGL_TRUE, EGL_NONE};
	c->image = t->image(t->dpy, EGL_NO_CONTEXT, EGL_NATIVE_BUFFER_ANDROID, t->client(b), attrs);
	if (c->image == EGL_NO_IMAGE_KHR) {
		logw("turn: EGL image: 0x%x", eglGetError());
		c->image = NULL;
		return NULL;
	}
	glGenTextures(1, &c->tex);
	glBindTexture(GL_TEXTURE_EXTERNAL_OES, c->tex);
	glTexParameteri(GL_TEXTURE_EXTERNAL_OES, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
	glTexParameteri(GL_TEXTURE_EXTERNAL_OES, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
	glTexParameteri(GL_TEXTURE_EXTERNAL_OES, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
	glTexParameteri(GL_TEXTURE_EXTERNAL_OES, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
	t->target(GL_TEXTURE_EXTERNAL_OES, (GLeglImageOES)c->image);
	c->buffer = b;
	return c;
}

static unsigned char *readback(output *o) {
	size_t row = (size_t)o->w * 4;
	unsigned char *pix = malloc(row * o->h);
	unsigned char *line = malloc(row);
	if (!pix || !line) {
		free(pix);
		free(line);
		return NULL;
	}
	glPixelStorei(GL_PACK_ALIGNMENT, 1);
	glReadPixels(0, 0, o->w, o->h, GL_RGBA, GL_UNSIGNED_BYTE, pix);
	for (int y = 0; y < o->h / 2; y++) {
		unsigned char *a = pix + (size_t)y * row, *b = pix + (size_t)(o->h - 1 - y) * row;
		memcpy(line, a, row);
		memcpy(a, b, row);
		memcpy(b, line, row);
	}
	free(line);
	return pix;
}

int turn_frame(turner *t, unsigned char **still) {
	AImage *img = NULL;
	if (AImageReader_acquireLatestImage(t->reader, &img) != AMEDIA_OK || !img) return 0;
	AHardwareBuffer *b = NULL;
	int64_t ns = 0;
	AImage_getTimestamp(img, &ns);
	int drew = 0;
	if (AImage_getHardwareBuffer(img, &b) == AMEDIA_OK && b) {
		for (int i = 0; i < t->outs; i++) {
			output *o = &t->out[i];
			if (!eglMakeCurrent(t->dpy, o->surface, o->surface, t->ctx)) continue;
			cached *c = texture(t, b);
			if (!c) break;
			glViewport(0, 0, o->w, o->h);
			glUseProgram(t->prog);
			glActiveTexture(GL_TEXTURE0);
			glBindTexture(GL_TEXTURE_EXTERNAL_OES, c->tex);
			glVertexAttribPointer(t->pos, 2, GL_FLOAT, GL_FALSE, 4 * sizeof(GLfloat), o->quad);
			glVertexAttribPointer(t->uv, 2, GL_FLOAT, GL_FALSE, 4 * sizeof(GLfloat), o->quad + 2);
			glEnableVertexAttribArray(t->pos);
			glEnableVertexAttribArray(t->uv);
			glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
			if (i == 0 && still && !*still) *still = readback(o);
			t->when(t->dpy, o->surface, ns);
			eglSwapBuffers(t->dpy, o->surface);
			drew = 1;
		}
	}
	AImage_delete(img);
	return drew;
}

void turn_close(turner *t) {
	if (!t) return;
	if (t->dpy != EGL_NO_DISPLAY && t->dpy) {
		eglMakeCurrent(t->dpy, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
		for (int i = 0; i < CACHE; i++)
			if (t->cache[i].image && t->unimage) t->unimage(t->dpy, t->cache[i].image);
		for (int i = 0; i < 2; i++)
			if (t->out[i].surface) eglDestroySurface(t->dpy, t->out[i].surface);
		if (t->ctx) eglDestroyContext(t->dpy, t->ctx);
	}
	if (t->reader) AImageReader_delete(t->reader);
	free(t);
}
