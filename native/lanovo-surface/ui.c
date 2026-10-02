#include <stdlib.h>
#include <string.h>
#include <time.h>

#include <EGL/egl.h>
#include <GLES2/gl2.h>
#include <android/log.h>
#include <sys/system_properties.h>

#include "protocol.h"
#include "ui.h"

#define TAG "lanovo-surface"
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)
#define logi(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)

#define ES3_BIT 0x40

static const char es_header[] = "#version 300 es\nprecision highp float;\nprecision highp int;\n";

static double now_us(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec * 1e6 + ts.tv_nsec / 1e3;
}

static const char *body(const char *src) {
	const char *v = strstr(src, "#version");
	if (!v) return src;
	const char *nl = strchr(v, '\n');
	return nl ? nl + 1 : src;
}

static GLuint shader(GLenum kind, const char *src) {
	GLuint s = glCreateShader(kind);
	const char *parts[2] = {es_header, body(src)};
	glShaderSource(s, 2, parts, NULL);
	glCompileShader(s);
	GLint ok = 0;
	glGetShaderiv(s, GL_COMPILE_STATUS, &ok);
	if (!ok) {
		char log[1024];
		glGetShaderInfoLog(s, sizeof log, NULL, log);
		logw("ui shader: %s", log);
		glDeleteShader(s);
		return 0;
	}
	return s;
}

static GLuint program(const char *vs, const char *fs) {
	GLuint v = shader(GL_VERTEX_SHADER, vs), f = shader(GL_FRAGMENT_SHADER, fs);
	if (!v || !f) {
		if (v) glDeleteShader(v);
		if (f) glDeleteShader(f);
		return 0;
	}
	GLuint p = glCreateProgram();
	glAttachShader(p, v);
	glAttachShader(p, f);
	glLinkProgram(p);
	glDeleteShader(v);
	glDeleteShader(f);
	GLint ok = 0;
	glGetProgramiv(p, GL_LINK_STATUS, &ok);
	if (!ok) {
		char log[1024];
		glGetProgramInfoLog(p, sizeof log, NULL, log);
		logw("ui link: %s", log);
		glDeleteProgram(p);
		return 0;
	}
	return p;
}

typedef struct {
	uint32_t id;
	GLuint tex;
	int w, h;
} uitex;

static uitex *texture_for(uitex *t, uint32_t id) {
	for (int i = 0; i < UI_TEXTURES; i++)
		if (t[i].tex && t[i].id == id) return &t[i];
	return NULL;
}

static void upload(uitex *t, const ui_upload *up) {
	uitex *x = texture_for(t, up->id);
	if (!x) {
		for (int i = 0; i < UI_TEXTURES && !x; i++)
			if (!t[i].tex) x = &t[i];
		if (!x) {
			static int warned;
			if (!warned++) logw("ui: all %d texture slots in use, image %u not uploaded", UI_TEXTURES, up->id);
			return;
		}
		x->id = up->id;
		glGenTextures(1, &x->tex);
		x->w = x->h = 0;
	}
	glActiveTexture(GL_TEXTURE0);
	glBindTexture(GL_TEXTURE_2D, x->tex);
	if (x->w != up->w || x->h != up->h) {
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
		glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, up->w, up->h, 0, GL_RGBA, GL_UNSIGNED_BYTE, NULL);
		x->w = up->w;
		x->h = up->h;
	}
	if (up->pix && up->rw > 0 && up->rh > 0)
		glTexSubImage2D(GL_TEXTURE_2D, 0, up->x, up->y, up->rw, up->rh, GL_RGBA, GL_UNSIGNED_BYTE, up->pix);
}

static void projection(float m[16], int rot, int w, int h) {
	memset(m, 0, 16 * sizeof(float));
	m[10] = -1;
	m[15] = 1;
	float sx = 2.0f / w, sy = 2.0f / h;
	switch (rot & 3) {
	case 1:
		m[4] = sx, m[12] = -1, m[1] = sy, m[13] = -1;
		break;
	case 2:
		m[0] = -sx, m[12] = 1, m[5] = sy, m[13] = -1;
		break;
	case 3:
		m[4] = -sx, m[12] = 1, m[1] = -sy, m[13] = 1;
		break;
	default:
		m[0] = sx, m[12] = -1, m[5] = -sy, m[13] = 1;
	}
}

static void native(int rot, int w, int h, int vx, int vy, int *nx, int *ny) {
	switch (rot & 3) {
	case 1:
		*nx = vy, *ny = h - vx;
		break;
	case 2:
		*nx = w - vx, *ny = h - vy;
		break;
	case 3:
		*nx = w - vy, *ny = vx;
		break;
	default:
		*nx = vx, *ny = vy;
	}
}

static void scissor(int rot, int w, int h, const uint32_t *r) {
	int ax, ay, bx, by;
	native(rot, w, h, (int)r[2], (int)r[3], &ax, &ay);
	native(rot, w, h, (int)(r[2] + r[4]), (int)(r[3] + r[5]), &bx, &by);
	int x0 = ax < bx ? ax : bx, x1 = ax < bx ? bx : ax;
	int y0 = ay < by ? ay : by, y1 = ay < by ? by : ay;
	glScissor(x0, h - y1, x1 - x0, y1 - y0);
}

static void *render(void *arg) {
	uiview *u = arg;
	EGLDisplay dpy = eglGetDisplay(EGL_DEFAULT_DISPLAY);
	eglInitialize(dpy, NULL, NULL);
	EGLint attrs[] = {EGL_SURFACE_TYPE, EGL_WINDOW_BIT, EGL_RENDERABLE_TYPE, ES3_BIT, EGL_RED_SIZE, 8, EGL_GREEN_SIZE, 8,
		EGL_BLUE_SIZE, 8, EGL_ALPHA_SIZE, 8, EGL_NONE};
	EGLConfig cfg;
	EGLint n = 0;
	eglChooseConfig(dpy, attrs, &cfg, 1, &n);
	EGLSurface surf = n > 0 ? eglCreateWindowSurface(dpy, cfg, u->win, NULL) : EGL_NO_SURFACE;
	EGLint ctxattrs[] = {EGL_CONTEXT_CLIENT_VERSION, 3, EGL_NONE};
	EGLContext ctx = n > 0 ? eglCreateContext(dpy, cfg, EGL_NO_CONTEXT, ctxattrs) : EGL_NO_CONTEXT;
	if (surf == EGL_NO_SURFACE || ctx == EGL_NO_CONTEXT || !eglMakeCurrent(dpy, surf, surf, ctx)) {
		logw("ui: surface/context failed 0x%x", eglGetError());
		pthread_mutex_lock(&u->mu);
		while (!u->closing) pthread_cond_wait(&u->cv, &u->mu);
		pthread_mutex_unlock(&u->mu);
		return NULL;
	}
	eglSwapInterval(dpy, 1);

	char prop[PROP_VALUE_MAX] = "";
	__system_property_get("debug.lanovo.uitime", prop);
	int timed = prop[0] == '1';

	GLuint progs[UI_PROGRAMS] = {0};
	GLint loc_mvp[UI_PROGRAMS], loc_tex[UI_PROGRAMS], loc_tm[UI_PROGRAMS], loc_tm2[UI_PROGRAMS];
	uitex textures[UI_TEXTURES];
	memset(textures, 0, sizeof textures);
	GLuint white;
	glGenTextures(1, &white);
	glBindTexture(GL_TEXTURE_2D, white);
	static const uint8_t one[4] = {255, 255, 255, 255};
	glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, 1, 1, 0, GL_RGBA, GL_UNSIGNED_BYTE, one);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST);

	float *quads = NULL;
	int quadcap = 0;
	uint32_t *runs = NULL;
	int runcap = 0;
	uint32_t *idx = NULL;
	int idxcap = 0;
	float mvp[16];
	double spent = 0, finished = 0;
	int frames = 0;

	for (;;) {
		pthread_mutex_lock(&u->mu);
		while (!u->dirty && !u->closing) pthread_cond_wait(&u->cv, &u->mu);
		if (u->closing) {
			pthread_mutex_unlock(&u->mu);
			break;
		}
		char *vs[UI_PROGRAMS], *fs[UI_PROGRAMS];
		memcpy(vs, u->vs, sizeof vs);
		memcpy(fs, u->fs, sizeof fs);
		memset(u->vs, 0, sizeof u->vs);
		memset(u->fs, 0, sizeof u->fs);
		ui_upload *ups = u->ups;
		int nups = u->nups;
		u->ups = NULL;
		u->nups = u->upcap = 0;
		if (u->nquads > quadcap) {
			free(quads);
			quadcap = u->nquads;
			quads = malloc((size_t)quadcap * UI_QUAD_FLOATS * sizeof(float));
			if (!quads) quadcap = 0;
		}
		int nquads = u->nquads <= quadcap ? u->nquads : 0;
		if (nquads) memcpy(quads, u->quads, (size_t)nquads * UI_QUAD_FLOATS * sizeof(float));
		if (u->nruns > runcap) {
			free(runs);
			runcap = u->nruns;
			runs = malloc((size_t)runcap * UI_RUN_WORDS * sizeof(uint32_t));
			if (!runs) runcap = 0;
		}
		int nruns = u->nruns <= runcap ? u->nruns : 0;
		if (nruns) memcpy(runs, u->runs, (size_t)nruns * UI_RUN_WORDS * sizeof(uint32_t));
		float clear[4];
		memcpy(clear, u->clear, sizeof clear);
		int rot = u->rot;
		uint32_t frame = u->frame;
		u->dirty = 0;
		pthread_mutex_unlock(&u->mu);

		double t0 = now_us();
		for (int i = 0; i < UI_PROGRAMS; i++) {
			if (!vs[i] || !fs[i]) {
				free(vs[i]);
				free(fs[i]);
				continue;
			}
			GLuint p = program(vs[i], fs[i]);
			free(vs[i]);
			free(fs[i]);
			if (!p) continue;
			if (progs[i]) glDeleteProgram(progs[i]);
			progs[i] = p;
			loc_mvp[i] = glGetUniformLocation(p, "mvp");
			loc_tex[i] = glGetUniformLocation(p, "tex");
			if (loc_tex[i] < 0) loc_tex[i] = glGetUniformLocation(p, "tex_smp");
			loc_tm[i] = glGetUniformLocation(p, "tm");
			loc_tm2[i] = glGetUniformLocation(p, "tm2");
		}
		for (int i = 0; i < nups; i++) {
			upload(textures, &ups[i]);
			free(ups[i].pix);
		}
		free(ups);

		if (nquads * 6 > idxcap) {
			free(idx);
			idxcap = nquads * 6;
			idx = malloc((size_t)idxcap * sizeof(uint32_t));
			if (!idx) idxcap = 0;
			for (int q = 0; idx && q < nquads; q++) {
				uint32_t b = (uint32_t)q * 4;
				uint32_t *o = idx + q * 6;
				o[0] = b, o[1] = b + 1, o[2] = b + 2, o[3] = b, o[4] = b + 2, o[5] = b + 3;
			}
		}

		projection(mvp, rot, u->w, u->h);
		glViewport(0, 0, u->w, u->h);
		glDisable(GL_SCISSOR_TEST);
		glClearColor(clear[0], clear[1], clear[2], clear[3]);
		glClear(GL_COLOR_BUFFER_BIT);
		glEnable(GL_BLEND);
		glBlendFuncSeparate(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA, GL_ONE, GL_ONE_MINUS_SRC_ALPHA);
		GLsizei stride = UI_VERTEX_FLOATS * sizeof(float);
		glEnableVertexAttribArray(0);
		glEnableVertexAttribArray(1);
		glEnableVertexAttribArray(2);
		glVertexAttribPointer(0, 3, GL_FLOAT, GL_FALSE, stride, quads);
		glVertexAttribPointer(1, 2, GL_FLOAT, GL_FALSE, stride, quads ? quads + 3 : NULL);
		glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, stride, quads ? quads + 5 : NULL);
		GLuint bound = 0;
		for (int r = 0; r < nruns && idx; r++) {
			const uint32_t *run = runs + r * UI_RUN_WORDS;
			uint32_t slot = run[0], texid = run[1], first = run[6], count = run[7];
			if (slot >= UI_PROGRAMS || !progs[slot] || first + count > (uint32_t)nquads || !count) continue;
			if (progs[slot] != bound) {
				bound = progs[slot];
				glUseProgram(bound);
				glUniformMatrix4fv(loc_mvp[slot], 1, GL_FALSE, mvp);
				glUniform1i(loc_tex[slot], 0);
			}
			if (loc_tm[slot] >= 0) glUniformMatrix4fv(loc_tm[slot], 1, GL_FALSE, (const float *)(run + 8));
			if (loc_tm2[slot] >= 0) glUniformMatrix4fv(loc_tm2[slot], 1, GL_FALSE, (const float *)(run + 24));
			uitex *t = texid ? texture_for(textures, texid) : NULL;
			glActiveTexture(GL_TEXTURE0);
			glBindTexture(GL_TEXTURE_2D, t ? t->tex : white);
			if (run[4] && run[5]) {
				glEnable(GL_SCISSOR_TEST);
				scissor(rot, u->w, u->h, run);
			} else {
				glDisable(GL_SCISSOR_TEST);
			}
			glDrawElements(GL_TRIANGLES, (GLsizei)count * 6, GL_UNSIGNED_INT, idx + first * 6);
		}
		double t1 = now_us();
		if (timed) glFinish();
		double t2 = now_us();
		pthread_mutex_lock(&u->mu);
		if (u->read_want && u->read_buf) {
			glReadPixels(0, 0, u->w, u->h, GL_RGBA, GL_UNSIGNED_BYTE, u->read_buf);
			u->read_ok = glGetError() == GL_NO_ERROR;
			u->read_want = 0;
			u->read_done = 1;
			pthread_cond_broadcast(&u->cv);
		}
		pthread_mutex_unlock(&u->mu);
		eglSwapBuffers(dpy, surf);

		pthread_mutex_lock(&u->mu);
		u->done = frame;
		u->submit_us = (uint32_t)(t1 - t0);
		u->finish_us = (uint32_t)(t2 - t0);
		pthread_cond_broadcast(&u->cv);
		pthread_mutex_unlock(&u->mu);

		if (timed) {
			spent += t1 - t0;
			finished += t2 - t0;
			if (++frames >= 120) {
				logi("ui %dx%d quads %d runs %d ms/frame: submit %.2f finish %.2f", u->w, u->h, nquads, nruns,
					spent / frames / 1e3, finished / frames / 1e3);
				spent = finished = 0;
				frames = 0;
			}
		}
	}

	for (int i = 0; i < UI_PROGRAMS; i++)
		if (progs[i]) glDeleteProgram(progs[i]);
	for (int i = 0; i < UI_TEXTURES; i++)
		if (textures[i].tex) glDeleteTextures(1, &textures[i].tex);
	glDeleteTextures(1, &white);
	free(quads);
	free(runs);
	free(idx);
	eglMakeCurrent(dpy, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
	eglDestroyContext(dpy, ctx);
	eglDestroySurface(dpy, surf);
	return NULL;
}

int ui_open(uiview *u, ANativeWindow *win, int w, int h) {
	memset(u, 0, sizeof *u);
	u->win = win;
	u->w = w;
	u->h = h;
	pthread_mutex_init(&u->mu, NULL);
	pthread_cond_init(&u->cv, NULL);
	if (pthread_create(&u->th, NULL, render, u)) return ERR_MEMORY;
	u->started = 1;
	return OK;
}

void ui_program(uiview *u, int slot, const char *vs, int vslen, const char *fs, int fslen) {
	if (slot < 0 || slot >= UI_PROGRAMS) return;
	char *v = malloc((size_t)vslen + 1), *f = malloc((size_t)fslen + 1);
	if (!v || !f) {
		free(v);
		free(f);
		return;
	}
	memcpy(v, vs, (size_t)vslen);
	v[vslen] = 0;
	memcpy(f, fs, (size_t)fslen);
	f[fslen] = 0;
	pthread_mutex_lock(&u->mu);
	free(u->vs[slot]);
	free(u->fs[slot]);
	u->vs[slot] = v;
	u->fs[slot] = f;
	pthread_mutex_unlock(&u->mu);
}

void ui_texture(uiview *u, uint32_t id, int w, int h, int x, int y, int rw, int rh, const uint8_t *pix) {
	ui_upload up = {.id = id, .w = w, .h = h, .x = x, .y = y, .rw = rw, .rh = rh};
	if (pix && rw > 0 && rh > 0) {
		size_t n = (size_t)rw * rh * 4;
		up.pix = malloc(n);
		if (!up.pix) return;
		memcpy(up.pix, pix, n);
	}
	pthread_mutex_lock(&u->mu);
	if (u->nups == u->upcap) {
		int cap = u->upcap ? u->upcap * 2 : 8;
		ui_upload *grown = realloc(u->ups, (size_t)cap * sizeof *grown);
		if (!grown) {
			pthread_mutex_unlock(&u->mu);
			free(up.pix);
			return;
		}
		u->ups = grown;
		u->upcap = cap;
	}
	u->ups[u->nups++] = up;
	pthread_mutex_unlock(&u->mu);
}

uint32_t ui_frame(uiview *u, int rot, const float clear[4], const float *quads, int nquads, const uint32_t *runs, int nruns) {
	pthread_mutex_lock(&u->mu);
	if (nquads > u->quadcap) {
		float *q = realloc(u->quads, (size_t)nquads * UI_QUAD_FLOATS * sizeof(float));
		if (q) {
			u->quads = q;
			u->quadcap = nquads;
		}
	}
	if (nruns > u->runcap) {
		uint32_t *r = realloc(u->runs, (size_t)nruns * UI_RUN_WORDS * sizeof(uint32_t));
		if (r) {
			u->runs = r;
			u->runcap = nruns;
		}
	}
	u->nquads = nquads <= u->quadcap ? nquads : 0;
	u->nruns = nruns <= u->runcap ? nruns : 0;
	if (u->nquads) memcpy(u->quads, quads, (size_t)u->nquads * UI_QUAD_FLOATS * sizeof(float));
	if (u->nruns) memcpy(u->runs, runs, (size_t)u->nruns * UI_RUN_WORDS * sizeof(uint32_t));
	memcpy(u->clear, clear, sizeof u->clear);
	u->rot = rot;
	uint32_t frame = ++u->frame;
	u->dirty = 1;
	pthread_cond_broadcast(&u->cv);
	pthread_mutex_unlock(&u->mu);
	return frame;
}

void ui_wait(uiview *u, uint32_t frame, uint32_t *submit_us, uint32_t *finish_us) {
	pthread_mutex_lock(&u->mu);
	while ((int32_t)(u->done - frame) < 0 && !u->closing) pthread_cond_wait(&u->cv, &u->mu);
	*submit_us = u->submit_us;
	*finish_us = u->finish_us;
	pthread_mutex_unlock(&u->mu);
}

int ui_read(uiview *u, uint8_t *out) {
	if (!u->started) return ERR_ARGS;
	struct timespec until;
	clock_gettime(CLOCK_REALTIME, &until);
	until.tv_sec += 5;
	pthread_mutex_lock(&u->mu);
	u->read_buf = out;
	u->read_want = 1;
	u->read_done = 0;
	u->dirty = 1;
	pthread_cond_broadcast(&u->cv);
	int late = 0;
	while (!u->read_done && !u->closing && !late) late = pthread_cond_timedwait(&u->cv, &u->mu, &until) != 0;
	int ok = u->read_done && u->read_ok;
	u->read_want = 0;
	u->read_buf = NULL;
	pthread_mutex_unlock(&u->mu);
	return ok ? OK : ERR_ARGS;
}

void ui_close(uiview *u) {
	if (!u->started) return;
	pthread_mutex_lock(&u->mu);
	u->closing = 1;
	pthread_cond_broadcast(&u->cv);
	pthread_mutex_unlock(&u->mu);
	pthread_join(u->th, NULL);
	for (int i = 0; i < UI_PROGRAMS; i++) {
		free(u->vs[i]);
		free(u->fs[i]);
	}
	for (int i = 0; i < u->nups; i++) free(u->ups[i].pix);
	free(u->ups);
	free(u->quads);
	free(u->runs);
	pthread_mutex_destroy(&u->mu);
	pthread_cond_destroy(&u->cv);
	u->started = 0;
}
