#include <errno.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include <EGL/egl.h>
#include <GLES2/gl2.h>
#include <android/log.h>
#include <sys/system_properties.h>

#include "gl.h"
#include "protocol.h"

#define TAG "lanovo-surface"
#define logw(...) __android_log_print(ANDROID_LOG_WARN, TAG, __VA_ARGS__)
#define logi(...) __android_log_print(ANDROID_LOG_INFO, TAG, __VA_ARGS__)

#define UNIT_SRC 4
#define UNIT_O0 5
#define UNIT_O1 6
#define UNIT_PRE 7
#define UNIT_FEED 8
#define UNIT_SPLAT_A 9
#define UNIT_SPLAT_B 10
#define UNIT_LINES 11
#define HALF_FLOAT 0x140B
#define RGBA16F 0x881A
#define ES3_BIT 0x40

static const char *vertex_src =
	"attribute vec2 p;\n"
	"varying highp vec2 vtex;\n"
	"varying highp vec2 vgl;\n"
	"varying highp vec2 vpre0;\n"
	"varying highp vec2 vpre1;\n"
	"varying highp vec2 vpre2;\n"
	"varying highp vec2 vpre3;\n"
	"void main() {\n"
	"	vtex = vec2(p.x * 0.5 + 0.5, 0.5 - p.y * 0.5);\n"
	"	vgl = p * 0.5 + 0.5;\n"
	"	vpre0 = vec2(vtex.x, 0.5 / 8.0);\n"
	"	vpre1 = vec2(vtex.x, 1.5 / 8.0);\n"
	"	vpre2 = vec2(vtex.x, 2.5 / 8.0);\n"
	"	vpre3 = vec2(vtex.x, 3.5 / 8.0);\n"
	"	gl_Position = vec4(p, 0.0, 1.0);\n"
	"}\n";

static const char *prelude =
	"precision mediump float;\n"
	"uniform highp vec2 res;\n"
	"uniform highp float u[128];\n"
	"uniform sampler2D t0;\n"
	"uniform sampler2D t1;\n"
	"uniform sampler2D t2;\n"
	"uniform sampler2D t3;\n"
	"uniform sampler2D pre;\n"
	"uniform sampler2D feed;\n"
	"uniform sampler2D splatA;\n"
	"uniform sampler2D splatB;\n"
	"uniform sampler2D lines;\n"
	"#ifdef LIGHT\n"
	"vec4 linesAt(highp vec2 uv) { return texture2D(lines, uv, 3.0); }\n"
	"#else\n"
	"vec4 linesAt(highp vec2 uv) { return texture2D(lines, uv); }\n"
	"#endif\n"
	"#ifdef SPLAT\n"
	"varying mediump vec4 vcol;\n"
	"varying highp float vsize;\n"
	"#endif\n"
	"uniform sampler2D glow0;\n"
	"uniform sampler2D glow1;\n"
	"uniform float glowAmount;\n"
	"uniform float glowSecond;\n"
	"varying highp vec2 vtex;\n"
	"varying highp vec2 vgl;\n"
	"varying highp vec2 vpre0;\n"
	"varying highp vec2 vpre1;\n"
	"varying highp vec2 vpre2;\n"
	"varying highp vec2 vpre3;\n"
	"highp vec2 pixel() { return vec2(gl_FragCoord.x, res.y - gl_FragCoord.y); }\n"
	"vec4 tex(sampler2D t, highp vec2 px) { return texture2D(t, px / res); }\n"
	"highp vec2 enc16(highp float v) { v = clamp(v, 0.0, 1.0) * 65535.0; highp float hi = floor(v / 256.0); return vec2(hi, floor(v - hi * 256.0)) / 255.0; }\n"
	"highp float dec16(highp vec2 c) { return (floor(c.x * 255.0 + 0.5) * 256.0 + floor(c.y * 255.0 + 0.5)) / 65535.0; }\n"
	"highp vec4 preAt(highp float x, float row) { return texture2D(pre, vec2(x, (row + 0.5) / 8.0)); }\n"
	"#ifdef LIGHT\n"
	"vec3 glow() { return vec3(0.0); }\n"
	"#else\n"
	"vec3 glow() {\n"
	"	if (glowAmount <= 0.0) return vec3(0.0);\n"
	"	return glowAmount * texture2D(glow0, vgl).rgb;\n"
	"}\n"
	"#endif\n";

static const char *point_vertex_src =
	"attribute vec2 pos;\n"
	"attribute float size;\n"
	"attribute vec4 col;\n"
	"attribute float shape;\n"
	"uniform highp vec2 res;\n"
	"uniform highp float scale;\n"
	"varying mediump vec4 vcol;\n"
	"varying mediump float vround;\n"
	"varying mediump float vdisc;\n"
	"varying highp float vsize;\n"
	"void main() {\n"
	"	vcol = vec4(col.rgb * col.a, col.a);\n"
	"	vround = mod(floor(shape / 2.0), 2.0);\n"
	"	vdisc = mod(floor(shape / 16.0), 2.0);\n"
	"	vsize = max(size * scale, 1.0);\n"
	"	gl_PointSize = vsize;\n"
	"	gl_Position = vec4(pos.x / res.x * 2.0 - 1.0, 1.0 - pos.y / res.y * 2.0, 0.0, 1.0);\n"
	"}\n";

static const char *point_src =
	"precision mediump float;\n"
	"varying vec4 vcol;\n"
	"varying float vround;\n"
	"varying float vdisc;\n"
	"varying highp float vsize;\n"
	"void main() {\n"
	"	float k = 1.0;\n"
	"	vec2 d = gl_PointCoord * 2.0 - 1.0;\n"
	"	if (vround > 0.5) { k = max(1.0 - dot(d, d), 0.0); k *= k; }\n"
	"	if (vdisc > 0.5) k = clamp((1.0 - length(d)) * vsize * 0.5 + 0.5, 0.0, 1.0);\n"
	"	gl_FragColor = vcol * k;\n"
	"}\n";

static const char *splat_vertex_src =
	"attribute vec2 pos;\n"
	"attribute float size;\n"
	"attribute vec4 col;\n"
	"attribute float shape;\n"
	"uniform highp vec2 res;\n"
	"uniform highp float scale;\n"
	"varying mediump vec4 vcol;\n"
	"varying highp float vsize;\n"
	"varying highp vec2 vtex;\n"
	"varying highp vec2 vgl;\n"
	"varying highp vec2 vpre0;\n"
	"varying highp vec2 vpre1;\n"
	"varying highp vec2 vpre2;\n"
	"varying highp vec2 vpre3;\n"
	"void main() {\n"
	"	vtex = vgl = vpre0 = vpre1 = vpre2 = vpre3 = vec2(0.0);\n"
	"	vcol = col;\n"
	"	vsize = max(size * scale, 1.0);\n"
	"	gl_PointSize = vsize;\n"
	"	gl_Position = vec4(pos.x / res.x * 2.0 - 1.0, 1.0 - pos.y / res.y * 2.0, 0.0, 1.0);\n"
	"}\n";

static const char *line_vertex_src =
	"attribute vec2 pos;\n"
	"attribute vec4 seg;\n"
	"attribute vec4 col;\n"
	"attribute float width;\n"
	"uniform highp vec2 res;\n"
	"varying highp vec4 vseg;\n"
	"varying mediump vec4 vcol;\n"
	"varying highp float vw;\n"
	"void main() {\n"
	"	vseg = seg;\n"
	"	vcol = vec4(col.rgb * col.a, col.a);\n"
	"	vw = width;\n"
	"	gl_Position = vec4(pos.x / res.x * 2.0 - 1.0, 1.0 - pos.y / res.y * 2.0, 0.0, 1.0);\n"
	"}\n";

static const char *line_src =
	"precision highp float;\n"
	"uniform vec2 res;\n"
	"varying vec4 vseg;\n"
	"varying mediump vec4 vcol;\n"
	"varying float vw;\n"
	"void main() {\n"
	"	vec2 p = vec2(gl_FragCoord.x, res.y - gl_FragCoord.y);\n"
	"	vec2 a = vseg.xy, d = vseg.zw - vseg.xy;\n"
	"	float t = clamp(dot(p - a, d) / max(dot(d, d), 1e-6), 0.0, 1.0);\n"
	"	float k = clamp(vw * 0.5 - length(p - a - d * t) + 0.5, 0.0, 1.0);\n"
	"	gl_FragColor = vcol * k;\n"
	"}\n";

static const char *quad_vertex_src =
	"attribute vec2 pos;\n"
	"attribute vec4 col;\n"
	"uniform highp vec2 res;\n"
	"varying mediump vec4 vcol;\n"
	"void main() {\n"
	"	vcol = vec4(col.rgb * col.a, col.a);\n"
	"	gl_Position = vec4(pos.x / res.x * 2.0 - 1.0, 1.0 - pos.y / res.y * 2.0, 0.0, 1.0);\n"
	"}\n";

static const char *quad_src =
	"precision mediump float;\n"
	"varying vec4 vcol;\n"
	"void main() { gl_FragColor = vcol; }\n";

static const char *down2_src =
	"precision mediump float;\n"
	"uniform sampler2D src;\n"
	"uniform highp vec2 texel;\n"
	"void main() { gl_FragColor = texture2D(src, gl_FragCoord.xy * 2.0 * texel); }\n";

static const char *add_src =
	"precision mediump float;\n"
	"uniform sampler2D src;\n"
	"uniform sampler2D more;\n"
	"uniform highp vec2 texel;\n"
	"uniform float second;\n"
	"void main() { highp vec2 uv = gl_FragCoord.xy * texel; gl_FragColor = texture2D(src, uv) + second * texture2D(more, uv); }\n";

static const char *blur_src =
	"precision mediump float;\n"
	"uniform sampler2D src;\n"
	"uniform highp vec2 texel;\n"
	"uniform highp vec2 dir;\n"
	"uniform float r;\n"
	"void main() {\n"
	"	highp vec2 uv = gl_FragCoord.xy * texel;\n"
	"	float n = 2.0 * r;\n"
	"	vec4 s = texture2D(src, uv) * (n + 1.0);\n"
	"	float tot = n + 1.0;\n"
	"	for (int k = 1; k < 256; k += 2) {\n"
	"		float i = float(k);\n"
	"		if (i > n) break;\n"
	"		float w0 = n + 1.0 - i;\n"
	"		float w1 = max(n - i, 0.0);\n"
	"		float w = w0 + w1;\n"
	"		highp vec2 o = dir * (i + w1 / w) * texel;\n"
	"		s += (texture2D(src, uv + o) + texture2D(src, uv - o)) * w;\n"
	"		tot += 2.0 * w;\n"
	"	}\n"
	"	gl_FragColor = s / tot;\n"
	"}\n";

static GLuint compile(GLenum kind, const char *a, const char *b, const char *c) {
	GLuint s = glCreateShader(kind);
	const char *parts[3] = {a, b, c};
	glShaderSource(s, c ? 3 : b ? 2 : 1, parts, NULL);
	glCompileShader(s);
	GLint ok = 0;
	glGetShaderiv(s, GL_COMPILE_STATUS, &ok);
	if (!ok) {
		char log[1024];
		glGetShaderInfoLog(s, sizeof log, NULL, log);
		logw("shader: %s", log);
		glDeleteShader(s);
		return 0;
	}
	return s;
}

static GLuint link_with(const char *vsrc, const char *a, const char *b, const char *c) {
	GLuint vs = compile(GL_VERTEX_SHADER, vsrc, NULL, NULL);
	GLuint fs = compile(GL_FRAGMENT_SHADER, a, b, c);
	if (!vs || !fs) {
		if (vs) glDeleteShader(vs);
		if (fs) glDeleteShader(fs);
		return 0;
	}
	GLuint p = glCreateProgram();
	glAttachShader(p, vs);
	glAttachShader(p, fs);
	glBindAttribLocation(p, 0, "p");
	glBindAttribLocation(p, 0, "pos");
	glBindAttribLocation(p, 1, "size");
	glBindAttribLocation(p, 2, "col");
	glBindAttribLocation(p, 3, "shape");
	glBindAttribLocation(p, 1, "seg");
	glBindAttribLocation(p, 3, "width");
	glLinkProgram(p);
	glDeleteShader(vs);
	glDeleteShader(fs);
	GLint ok = 0;
	glGetProgramiv(p, GL_LINK_STATUS, &ok);
	if (!ok) {
		char log[1024];
		glGetProgramInfoLog(p, sizeof log, NULL, log);
		logw("link: %s", log);
		glDeleteProgram(p);
		return 0;
	}
	return p;
}

static GLuint link(const char *a, const char *b, const char *c) { return link_with(vertex_src, a, b, c); }

typedef struct {
	GLuint fbo, tex;
	int w, h;
} target;

static void target_free(target *t) {
	if (t->fbo) glDeleteFramebuffers(1, &t->fbo);
	if (t->tex) glDeleteTextures(1, &t->tex);
	memset(t, 0, sizeof *t);
}

static int target_typed(target *t, int w, int h, GLint filter, GLenum type) {
	if (t->fbo && t->w == w && t->h == h) return 1;
	target_free(t);
	glGenTextures(1, &t->tex);
	glActiveTexture(GL_TEXTURE0 + UNIT_SRC);
	glBindTexture(GL_TEXTURE_2D, t->tex);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, filter);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, filter);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
	glTexImage2D(GL_TEXTURE_2D, 0, type == HALF_FLOAT ? RGBA16F : GL_RGBA, w, h, 0, GL_RGBA, type, NULL);
	glGenFramebuffers(1, &t->fbo);
	glBindFramebuffer(GL_FRAMEBUFFER, t->fbo);
	glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, t->tex, 0);
	int ok = glCheckFramebufferStatus(GL_FRAMEBUFFER) == GL_FRAMEBUFFER_COMPLETE;
	glBindFramebuffer(GL_FRAMEBUFFER, 0);
	if (!ok) {
		logw("gl: framebuffer %dx%d incomplete", w, h);
		target_free(t);
		return 0;
	}
	t->w = w;
	t->h = h;
	return 1;
}

static int target_make(target *t, int w, int h, GLint filter) { return target_typed(t, w, h, filter, GL_UNSIGNED_BYTE); }

static int target_fit(target *t, int w, int h) { return target_make(t, w, h, GL_LINEAR); }

enum { INTO_FINAL, INTO_FEED, INTO_LIGHT, INTO_SPLAT };

static int wanted(float flags, int pass) {
	int f = (int)flags;
	if (pass == INTO_FEED) return f & GL_POINT_FEED;
	if (pass == INTO_LIGHT) return f & GL_POINT_LIGHT;
	if (pass == INTO_SPLAT) return f & GL_POINT_SPLAT;
	return !(f & (GL_POINT_FEED | GL_POINT_SPLAT));
}

static void dots(GLuint prog, const float *pts, int n, int pass, int w, int h, float scale) {
	int any = 0;
	for (int i = 0; i < n && !any; i++) any = wanted(pts[i * GL_POINT_FLOATS + 7], pass);
	if (!any) return;
	glUseProgram(prog);
	glUniform2f(glGetUniformLocation(prog, "res"), (float)w, (float)h);
	glUniform1f(glGetUniformLocation(prog, "scale"), scale);
	glEnable(GL_BLEND);
	for (int a = 0; a < 4; a++) glEnableVertexAttribArray(a);
	GLsizei stride = GL_POINT_FLOATS * sizeof(float);
	for (int i = 0; i < n;) {
		float flags = pts[i * GL_POINT_FLOATS + 7];
		int j = i;
		while (j < n && pts[j * GL_POINT_FLOATS + 7] == flags && wanted(flags, pass)) j++;
		if (j > i) {
			const float *p = pts + i * GL_POINT_FLOATS;
			if (pass != INTO_SPLAT && ((int)flags & GL_POINT_OVER)) glBlendFunc(GL_ONE, GL_ONE_MINUS_SRC_ALPHA);
			else glBlendFunc(GL_ONE, GL_ONE);
			glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, stride, p);
			glVertexAttribPointer(1, 1, GL_FLOAT, GL_FALSE, stride, p + 2);
			glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, stride, p + 3);
			glVertexAttribPointer(3, 1, GL_FLOAT, GL_FALSE, stride, p + 7);
			glDrawArrays(GL_POINTS, 0, j - i);
			i = j;
		} else {
			i++;
		}
	}
	for (int a = 1; a < 4; a++) glDisableVertexAttribArray(a);
	glDisable(GL_BLEND);
}

static float *line_verts;
static int line_cap;

static void segments(GLuint prog, const float *segs, int n, int w, int h) {
	if (n <= 0) return;
	if (n * 6 > line_cap) {
		free(line_verts);
		line_cap = n * 6;
		line_verts = malloc((size_t)line_cap * 11 * sizeof(float));
		if (!line_verts) {
			line_cap = 0;
			return;
		}
	}
	static const float corner[6][2] = {{0, -1}, {1, -1}, {0, 1}, {0, 1}, {1, -1}, {1, 1}};
	float *v = line_verts;
	for (int i = 0; i < n; i++) {
		const float *s = segs + i * GL_LINE_FLOATS;
		float dx = s[2] - s[0], dy = s[3] - s[1];
		float len = dx * dx + dy * dy > 1e-8f ? sqrtf(dx * dx + dy * dy) : 1;
		float ux = dx / len, uy = dy / len, e = s[4] * 0.5f + 1.5f;
		for (int c = 0; c < 6; c++) {
			float al = corner[c][0], side = corner[c][1];
			float bx = al ? s[2] : s[0], by = al ? s[3] : s[1];
			float ext = al ? e : -e;
			*v++ = bx + ux * ext - uy * side * e;
			*v++ = by + uy * ext + ux * side * e;
			*v++ = s[0];
			*v++ = s[1];
			*v++ = s[2];
			*v++ = s[3];
			*v++ = s[5];
			*v++ = s[6];
			*v++ = s[7];
			*v++ = s[8];
			*v++ = s[4];
		}
	}
	glUseProgram(prog);
	glUniform2f(glGetUniformLocation(prog, "res"), (float)w, (float)h);
	glEnable(GL_BLEND);
	glBlendEquation(0x8008);
	glBlendFunc(GL_ONE, GL_ONE);
	GLsizei stride = 11 * sizeof(float);
	for (int a = 0; a < 4; a++) glEnableVertexAttribArray(a);
	glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, stride, line_verts);
	glVertexAttribPointer(1, 4, GL_FLOAT, GL_FALSE, stride, line_verts + 2);
	glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, stride, line_verts + 6);
	glVertexAttribPointer(3, 1, GL_FLOAT, GL_FALSE, stride, line_verts + 10);
	glDrawArrays(GL_TRIANGLES, 0, n * 6);
	for (int a = 1; a < 4; a++) glDisableVertexAttribArray(a);
	glBlendEquation(GL_FUNC_ADD);
	glDisable(GL_BLEND);
}

static float *quad_verts;
static int quad_cap;

static void fills(GLuint prog, const float *quads, int n, int w, int h) {
	if (n <= 0) return;
	if (n * 6 > quad_cap) {
		free(quad_verts);
		quad_cap = n * 6;
		quad_verts = malloc((size_t)quad_cap * 6 * sizeof(float));
		if (!quad_verts) {
			quad_cap = 0;
			return;
		}
	}
	static const int corner[6] = {0, 1, 2, 0, 2, 3};
	float *v = quad_verts;
	for (int i = 0; i < n; i++) {
		const float *q = quads + i * GL_QUAD_FLOATS;
		for (int c = 0; c < 6; c++) {
			*v++ = q[corner[c] * 2];
			*v++ = q[corner[c] * 2 + 1];
			*v++ = q[8];
			*v++ = q[9];
			*v++ = q[10];
			*v++ = q[11];
		}
	}
	glUseProgram(prog);
	glUniform2f(glGetUniformLocation(prog, "res"), (float)w, (float)h);
	glEnable(GL_BLEND);
	glBlendEquation(0x8008);
	glBlendFunc(GL_ONE, GL_ONE);
	GLsizei stride = 6 * sizeof(float);
	glEnableVertexAttribArray(0);
	glEnableVertexAttribArray(2);
	glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, stride, quad_verts);
	glVertexAttribPointer(2, 4, GL_FLOAT, GL_FALSE, stride, quad_verts + 2);
	glDrawArrays(GL_TRIANGLES, 0, n * 6);
	glDisableVertexAttribArray(2);
	glBlendEquation(GL_FUNC_ADD);
	glDisable(GL_BLEND);
}

static void into(target *t, int w, int h) {
	glBindFramebuffer(GL_FRAMEBUFFER, t ? t->fbo : 0);
	glViewport(0, 0, t ? t->w : w, t ? t->h : h);
	glClear(GL_COLOR_BUFFER_BIT);
}

static const GLfloat quad[] = {-1, -1, 1, -1, -1, 1, 1, 1};

static void draw(void) {
	glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, 0, quad);
	glEnableVertexAttribArray(0);
	glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
}

static void user(GLuint prog, int w, int h, const float *u, int nu, float amount, float second) {
	glUseProgram(prog);
	glUniform2f(glGetUniformLocation(prog, "res"), (float)w, (float)h);
	if (nu > 0) glUniform1fv(glGetUniformLocation(prog, "u"), nu, u);
	const char *names[GL_UNITS] = {"t0", "t1", "t2", "t3"};
	for (int i = 0; i < GL_UNITS; i++) {
		GLint loc = glGetUniformLocation(prog, names[i]);
		if (loc >= 0) glUniform1i(loc, i);
	}
	GLint loc;
	if ((loc = glGetUniformLocation(prog, "pre")) >= 0) glUniform1i(loc, UNIT_PRE);
	if ((loc = glGetUniformLocation(prog, "feed")) >= 0) glUniform1i(loc, UNIT_FEED);
	if ((loc = glGetUniformLocation(prog, "splatA")) >= 0) glUniform1i(loc, UNIT_SPLAT_A);
	if ((loc = glGetUniformLocation(prog, "splatB")) >= 0) glUniform1i(loc, UNIT_SPLAT_B);
	if ((loc = glGetUniformLocation(prog, "lines")) >= 0) glUniform1i(loc, UNIT_LINES);
	if ((loc = glGetUniformLocation(prog, "glow0")) >= 0) glUniform1i(loc, UNIT_O0);
	if ((loc = glGetUniformLocation(prog, "glow1")) >= 0) glUniform1i(loc, UNIT_O1);
	if ((loc = glGetUniformLocation(prog, "glowAmount")) >= 0) glUniform1f(loc, amount);
	if ((loc = glGetUniformLocation(prog, "glowSecond")) >= 0) glUniform1f(loc, second);
	draw();
}

static void sampled(GLuint prog, GLuint tex, int w, int h) {
	glUseProgram(prog);
	glActiveTexture(GL_TEXTURE0 + UNIT_SRC);
	glBindTexture(GL_TEXTURE_2D, tex);
	glUniform1i(glGetUniformLocation(prog, "src"), UNIT_SRC);
	glUniform2f(glGetUniformLocation(prog, "texel"), 1.0f / w, 1.0f / h);
}

static void blur(GLuint prog, target *o, target *tmp, float r) {
	into(tmp, 0, 0);
	sampled(prog, o->tex, o->w, o->h);
	glUniform2f(glGetUniformLocation(prog, "dir"), 1, 0);
	glUniform1f(glGetUniformLocation(prog, "r"), r);
	draw();
	into(o, 0, 0);
	sampled(prog, tmp->tex, tmp->w, tmp->h);
	glUniform2f(glGetUniformLocation(prog, "dir"), 0, 1);
	glUniform1f(glGetUniformLocation(prog, "r"), r);
	draw();
}

static int maxi(int a, int b) { return a > b ? a : b; }

static float radius_of(float radius, int h, int p) {
	float r = radius * h * (1 + p) / (float)(8 << p) + 0.5f;
	return (float)maxi((int)r, 1);
}

enum { ST_PRE, ST_LIGHT, ST_BLUR, ST_WAIT, ST_SCENE, ST_N };

typedef struct {
	int on, frames;
	double at, spent[ST_N];
} timing;

static double now_ms(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec * 1e3 + ts.tv_nsec / 1e6;
}

static void mark(timing *tm, int stage) {
	if (!tm->on) return;
	glFinish();
	double t = now_ms();
	if (stage >= 0) tm->spent[stage] += t - tm->at;
	tm->at = t;
}

static void report(timing *tm, const void *who, int w, int h) {
	if (!tm->on || ++tm->frames < 120) return;
	double n = tm->frames;
	logi("gl %p %dx%d ms/frame: pre %.2f light %.2f blur %.2f wait %.2f scene %.2f", who, w, h,
		tm->spent[ST_PRE] / n, tm->spent[ST_LIGHT] / n, tm->spent[ST_BLUR] / n, tm->spent[ST_WAIT] / n,
		tm->spent[ST_SCENE] / n);
	memset(tm->spent, 0, sizeof tm->spent);
	tm->frames = 0;
}

static void *render(void *arg) {
	glview *g = arg;
	EGLDisplay dpy = eglGetDisplay(EGL_DEFAULT_DISPLAY);
	eglInitialize(dpy, NULL, NULL);
	EGLint attrs[] = {EGL_SURFACE_TYPE, g->win ? EGL_WINDOW_BIT : EGL_PBUFFER_BIT, EGL_RENDERABLE_TYPE, ES3_BIT,
		EGL_RED_SIZE, 8, EGL_GREEN_SIZE, 8, EGL_BLUE_SIZE, 8, EGL_ALPHA_SIZE, 8, EGL_NONE};
	EGLConfig cfg;
	EGLint n = 0;
	eglChooseConfig(dpy, attrs, &cfg, 1, &n);
	EGLint pbattrs[] = {EGL_WIDTH, g->w, EGL_HEIGHT, g->h, EGL_NONE};
	EGLSurface surf = n <= 0 ? EGL_NO_SURFACE : g->win ? eglCreateWindowSurface(dpy, cfg, g->win, NULL) : eglCreatePbufferSurface(dpy, cfg, pbattrs);
	EGLint ctxattrs[] = {EGL_CONTEXT_CLIENT_VERSION, 3, EGL_NONE};
	EGLContext ctx = n > 0 ? eglCreateContext(dpy, cfg, EGL_NO_CONTEXT, ctxattrs) : EGL_NO_CONTEXT;
	if (surf == EGL_NO_SURFACE || ctx == EGL_NO_CONTEXT || !eglMakeCurrent(dpy, surf, surf, ctx)) {
		logw("gl: surface/context failed 0x%x", eglGetError());
		pthread_mutex_lock(&g->mu);
		g->compile_done = 1;
		pthread_cond_broadcast(&g->cv);
		while (!g->closing) pthread_cond_wait(&g->cv, &g->mu);
		pthread_mutex_unlock(&g->mu);
		return NULL;
	}

	GLuint textures[GL_UNITS];
	glGenTextures(GL_UNITS, textures);
	for (int i = 0; i < GL_UNITS; i++) {
		glActiveTexture(GL_TEXTURE0 + i);
		glBindTexture(GL_TEXTURE_2D, textures[i]);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
		glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
	}
	glClearColor(0, 0, 0, 1);
	GLuint down2 = link(down2_src, NULL, NULL);
	GLuint blurp = link(blur_src, NULL, NULL);
	GLuint addp = link(add_src, NULL, NULL);
	GLuint pointp = link_with(point_vertex_src, point_src, NULL, NULL);
	target feeds[2] = {{0}, {0}};
	target splats[2] = {{0}, {0}};
	target linet = {0};
	GLuint linep = link_with(line_vertex_src, line_src, NULL, NULL);
	float *segs = NULL;
	int nsegs = 0, segcap = 0;
	GLuint quadp = link_with(quad_vertex_src, quad_src, NULL, NULL);
	float *quads = NULL;
	int nquads = 0, quadcap = 0;
	GLuint sprog[2] = {0, 0};
	int newest = 0;
	GLuint fprog = 0;
	int pflags = 0;
	float *pts = NULL;
	int npts = 0, cap = 0;
	target o0 = {0}, t0 = {0}, o1 = {0}, t1 = {0}, pre = {0};
	GLuint prog = 0, lprog = 0, pprog = 0;
	float u[GL_VALUES];
	int nu = 0;
	gl_glow glow = {0};
	int W = g->w, H = g->h;
	char prop[PROP_VALUE_MAX] = "";
	__system_property_get("debug.lanovo.gltime", prop);
	timing tm = {.on = prop[0] == '1'};

	for (;;) {
		pthread_mutex_lock(&g->mu);
		while (!g->dirty && !g->closing) pthread_cond_wait(&g->cv, &g->mu);
		if (g->closing) {
			pthread_mutex_unlock(&g->mu);
			break;
		}
		gl_tex pend[GL_UNITS];
		memcpy(pend, g->pend, sizeof pend);
		memset(g->pend, 0, sizeof g->pend);
		char *src = g->src;
		int flags = g->src_flags;
		g->src = NULL;
		memcpy(u, g->u, sizeof u);
		nu = g->nu;
		glow = g->glow;
		if (g->npoints > cap) {
			free(pts);
			cap = g->npoints;
			pts = malloc((size_t)cap * GL_POINT_FLOATS * sizeof(float));
			if (!pts) cap = 0;
		}
		npts = g->npoints <= cap ? g->npoints : 0;
		if (npts) memcpy(pts, g->points, (size_t)npts * GL_POINT_FLOATS * sizeof(float));
		if (g->nlines > segcap) {
			free(segs);
			segcap = g->nlines;
			segs = malloc((size_t)segcap * GL_LINE_FLOATS * sizeof(float));
			if (!segs) segcap = 0;
		}
		nsegs = g->nlines <= segcap ? g->nlines : 0;
		if (nsegs) memcpy(segs, g->lines, (size_t)nsegs * GL_LINE_FLOATS * sizeof(float));
		if (g->nquads > quadcap) {
			free(quads);
			quadcap = g->nquads;
			quads = malloc((size_t)quadcap * GL_QUAD_FLOATS * sizeof(float));
			if (!quads) quadcap = 0;
		}
		nquads = g->nquads <= quadcap ? g->nquads : 0;
		if (nquads) memcpy(quads, g->quads, (size_t)nquads * GL_QUAD_FLOATS * sizeof(float));
		g->dirty = 0;
		pthread_mutex_unlock(&g->mu);

		for (int i = 0; i < GL_UNITS; i++) {
			if (!pend[i].pix) continue;
			glActiveTexture(GL_TEXTURE0 + i);
			glBindTexture(GL_TEXTURE_2D, textures[i]);
			glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, pend[i].w, pend[i].h, 0, GL_RGBA, GL_UNSIGNED_BYTE, pend[i].pix);
			free(pend[i].pix);
		}
		if (src) {
			GLuint p = link(prelude, src, NULL);
			GLuint lp = p && (flags & GL_WITH_LIGHT) ? link("#define LIGHT 1\n", prelude, src) : 0;
			GLuint pp = p && (flags & GL_WITH_PRE) ? link("#define PRE 1\n", prelude, src) : 0;
			GLuint fp = p && (flags & GL_WITH_FEED) ? link("#define FEED 1\n", prelude, src) : 0;
			GLuint sa = p && (flags & GL_WITH_SPLAT) ? link_with(splat_vertex_src, "#define SPLAT 1\n#define SPLAT_A 1\n", prelude, src) : 0;
			GLuint sb = p && (flags & GL_WITH_SPLAT) ? link_with(splat_vertex_src, "#define SPLAT 1\n#define SPLAT_B 1\n", prelude, src) : 0;
			free(src);
			int ok = p && (lp || !(flags & GL_WITH_LIGHT)) && (pp || !(flags & GL_WITH_PRE)) && (fp || !(flags & GL_WITH_FEED)) &&
				((sa && sb) || !(flags & GL_WITH_SPLAT));
			if (ok) {
				if (prog) glDeleteProgram(prog);
				if (lprog) glDeleteProgram(lprog);
				if (pprog) glDeleteProgram(pprog);
				if (fprog) glDeleteProgram(fprog);
				if (sprog[0]) glDeleteProgram(sprog[0]);
				if (sprog[1]) glDeleteProgram(sprog[1]);
				sprog[0] = sa;
				sprog[1] = sb;
				prog = p;
				lprog = lp;
				pprog = pp;
				fprog = fp;
				pflags = flags;
				target_free(&feeds[0]);
				target_free(&feeds[1]);
			} else {
				if (p) glDeleteProgram(p);
				if (lp) glDeleteProgram(lp);
				if (pp) glDeleteProgram(pp);
				if (fp) glDeleteProgram(fp);
				if (sa) glDeleteProgram(sa);
				if (sb) glDeleteProgram(sb);
			}
			pthread_mutex_lock(&g->mu);
			g->compiled = ok;
			g->compile_done = 1;
			int reading = g->read_want;
			pthread_cond_broadcast(&g->cv);
			pthread_mutex_unlock(&g->mu);
			if (!reading) continue;
		}
		if (!prog) {
			pthread_mutex_lock(&g->mu);
			if (g->read_want) {
				g->read_want = 0;
				g->read_ok = 0;
				g->read_done = 1;
				pthread_cond_broadcast(&g->cv);
			}
			pthread_mutex_unlock(&g->mu);
			continue;
		}

		mark(&tm, -1);
		if (pprog && target_make(&pre, W, GL_PRE_ROWS, GL_NEAREST)) {
			into(&pre, 0, 0);
			user(pprog, W, GL_PRE_ROWS, u, nu, 0, 0);
			glActiveTexture(GL_TEXTURE0 + UNIT_PRE);
			glBindTexture(GL_TEXTURE_2D, pre.tex);
		}
		if (linep && (pflags & GL_WITH_LINES) && target_make(&linet, W, H, GL_LINEAR)) {
			glClearColor(0, 0, 0, 0);
			into(&linet, 0, 0);
			glClearColor(0, 0, 0, 1);
			if (quadp) fills(quadp, quads, nquads, W, H);
			segments(linep, segs, nsegs, W, H);
			glActiveTexture(GL_TEXTURE0 + UNIT_LINES);
			glBindTexture(GL_TEXTURE_2D, linet.tex);
			glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR_MIPMAP_LINEAR);
			glGenerateMipmap(GL_TEXTURE_2D);
		}
		if (sprog[0] && sprog[1]) {
			int sw = pflags & GL_SPLAT_HALF ? maxi(W / 2, 2) : W, sh = pflags & GL_SPLAT_HALF ? maxi(H / 2, 2) : H;
			if (target_typed(&splats[0], sw, sh, GL_LINEAR, HALF_FLOAT) && target_typed(&splats[1], sw, sh, GL_LINEAR, HALF_FLOAT)) {
				glClearColor(0, 0, 0, 0);
				for (int k = 0; k < 2; k++) {
					into(&splats[k], 0, 0);
					dots(sprog[k], pts, npts, INTO_SPLAT, W, H, (float)sw / W);
				}
				glClearColor(0, 0, 0, 1);
				glActiveTexture(GL_TEXTURE0 + UNIT_SPLAT_A);
				glBindTexture(GL_TEXTURE_2D, splats[0].tex);
				glActiveTexture(GL_TEXTURE0 + UNIT_SPLAT_B);
				glBindTexture(GL_TEXTURE_2D, splats[1].tex);
			}
		}
		if (fprog) {
			int fw = pflags & GL_FEED_HALF ? maxi(W / 2, 2) : W, fh = pflags & GL_FEED_HALF ? maxi(H / 2, 2) : H;
			int fresh = !feeds[0].fbo;
			GLenum ft = pflags & GL_FEED_FLOAT ? HALF_FLOAT : GL_UNSIGNED_BYTE;
			if (target_typed(&feeds[0], fw, fh, GL_LINEAR, ft) && target_typed(&feeds[1], fw, fh, GL_LINEAR, ft)) {
				if (fresh) {
					glClearColor(0, 0, 0, 0);
					into(&feeds[0], 0, 0);
					into(&feeds[1], 0, 0);
					glClearColor(0, 0, 0, 1);
				}
				int next = 1 - newest;
				glActiveTexture(GL_TEXTURE0 + UNIT_FEED);
				glBindTexture(GL_TEXTURE_2D, feeds[newest].tex);
				into(&feeds[next], 0, 0);
				user(fprog, fw, fh, u, nu, 0, 0);
				if (pointp) dots(pointp, pts, npts, INTO_FEED, W, H, (float)fw / W);
				newest = next;
				glActiveTexture(GL_TEXTURE0 + UNIT_FEED);
				glBindTexture(GL_TEXTURE_2D, feeds[newest].tex);
			}
		}
		mark(&tm, ST_PRE);

		int passes = glow.passes > 1 ? 2 : 1;
		int glowing = lprog && glow.amount > 0 && down2 && blurp && addp &&
			target_fit(&o0, maxi(W / 8, 2), maxi(H / 8, 2)) && target_fit(&t0, o0.w, o0.h) &&
			(passes < 2 || (target_fit(&o1, maxi(o0.w / 2, 2), maxi(o0.h / 2, 2)) && target_fit(&t1, o1.w, o1.h)));
		if (glowing) {
			into(&o0, 0, 0);
			user(lprog, o0.w, o0.h, u, nu, 0, 0);
			if (pointp) dots(pointp, pts, npts, INTO_LIGHT, W, H, (float)o0.w / W);
			if (passes > 1) {
				into(&o1, 0, 0);
				sampled(down2, o0.tex, o0.w, o0.h);
				draw();
			}
			mark(&tm, ST_LIGHT);
			blur(blurp, &o0, &t0, radius_of(glow.radius, H, 0));
			target *sum = &o0;
			if (passes > 1) {
				blur(blurp, &o1, &t1, radius_of(glow.radius, H, 1));
				into(&t0, 0, 0);
				sampled(addp, o0.tex, o0.w, o0.h);
				glActiveTexture(GL_TEXTURE0 + UNIT_O1);
				glBindTexture(GL_TEXTURE_2D, o1.tex);
				glUniform1i(glGetUniformLocation(addp, "more"), UNIT_O1);
				glUniform1f(glGetUniformLocation(addp, "second"), 1.0f / 1.6f);
				draw();
				sum = &t0;
			}
			glActiveTexture(GL_TEXTURE0 + UNIT_O0);
			glBindTexture(GL_TEXTURE_2D, sum->tex);
			mark(&tm, ST_BLUR);
		}

		into(NULL, W, H);
		mark(&tm, ST_WAIT);
		user(prog, W, H, u, nu, glowing ? glow.amount : 0, 0);
		if (pointp) dots(pointp, pts, npts, INTO_FINAL, W, H, 1);
		mark(&tm, ST_SCENE);
		report(&tm, g, W, H);
		pthread_mutex_lock(&g->mu);
		if (g->read_want) {
			glReadPixels(0, 0, W, H, GL_RGBA, GL_UNSIGNED_BYTE, g->read_buf);
			g->read_want = 0;
			g->read_ok = glGetError() == GL_NO_ERROR;
			g->read_done = 1;
			pthread_cond_broadcast(&g->cv);
		}
		pthread_mutex_unlock(&g->mu);
		eglSwapBuffers(dpy, surf);
	}

	target_free(&o0);
	target_free(&t0);
	target_free(&o1);
	target_free(&t1);
	target_free(&pre);
	target_free(&feeds[0]);
	target_free(&feeds[1]);
	target_free(&splats[0]);
	target_free(&splats[1]);
	target_free(&linet);
	if (linep) glDeleteProgram(linep);
	free(segs);
	if (quadp) glDeleteProgram(quadp);
	free(quads);
	if (sprog[0]) glDeleteProgram(sprog[0]);
	if (sprog[1]) glDeleteProgram(sprog[1]);
	free(pts);
	if (fprog) glDeleteProgram(fprog);
	if (pointp) glDeleteProgram(pointp);
	if (prog) glDeleteProgram(prog);
	if (lprog) glDeleteProgram(lprog);
	if (pprog) glDeleteProgram(pprog);
	if (down2) glDeleteProgram(down2);
	if (blurp) glDeleteProgram(blurp);
	if (addp) glDeleteProgram(addp);
	glDeleteTextures(GL_UNITS, textures);
	eglMakeCurrent(dpy, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
	eglDestroyContext(dpy, ctx);
	eglDestroySurface(dpy, surf);
	return NULL;
}

int gl_open(glview *g, ANativeWindow *win, int w, int h) {
	memset(g, 0, sizeof *g);
	g->win = win;
	g->w = w;
	g->h = h;
	pthread_mutex_init(&g->mu, NULL);
	pthread_cond_init(&g->cv, NULL);
	if (pthread_create(&g->th, NULL, render, g) != 0) return ERR_SURFACE;
	g->started = 1;
	return OK;
}

void gl_texture(glview *g, int unit, int w, int h, const uint8_t *pix) {
	if (unit < 0 || unit >= GL_UNITS) return;
	size_t n = (size_t)w * h * 4;
	uint8_t *copy = malloc(n);
	if (!copy) return;
	memcpy(copy, pix, n);
	pthread_mutex_lock(&g->mu);
	free(g->pend[unit].pix);
	g->pend[unit] = (gl_tex){w, h, copy};
	pthread_mutex_unlock(&g->mu);
}

int gl_program(glview *g, const char *src, int len, int flags) {
	char *s = malloc(len + 1);
	if (!s) return ERR_MEMORY;
	memcpy(s, src, len);
	s[len] = 0;
	struct timespec until;
	clock_gettime(CLOCK_REALTIME, &until);
	until.tv_sec += 5;
	pthread_mutex_lock(&g->mu);
	free(g->src);
	g->src = s;
	g->src_flags = flags;
	g->compile_done = 0;
	g->dirty = 1;
	pthread_cond_broadcast(&g->cv);
	int err = 0;
	while (!g->compile_done && err != ETIMEDOUT) err = pthread_cond_timedwait(&g->cv, &g->mu, &until);
	int ok = g->compile_done && g->compiled;
	pthread_mutex_unlock(&g->mu);
	return ok ? OK : ERR_SHADER;
}

void gl_values(glview *g, const float *u, int n, gl_glow glow) {
	if (n > GL_VALUES) n = GL_VALUES;
	pthread_mutex_lock(&g->mu);
	memcpy(g->u, u, n * sizeof(float));
	g->nu = n;
	g->glow = glow;
	g->dirty = 1;
	pthread_cond_broadcast(&g->cv);
	pthread_mutex_unlock(&g->mu);
}

void gl_points(glview *g, const float *p, int n) {
	float *copy = n ? malloc((size_t)n * GL_POINT_FLOATS * sizeof(float)) : NULL;
	if (n && !copy) return;
	if (n) memcpy(copy, p, (size_t)n * GL_POINT_FLOATS * sizeof(float));
	pthread_mutex_lock(&g->mu);
	free(g->points);
	g->points = copy;
	g->npoints = n;
	pthread_mutex_unlock(&g->mu);
}

void gl_lines(glview *g, const float *p, int n) {
	float *copy = n ? malloc((size_t)n * GL_LINE_FLOATS * sizeof(float)) : NULL;
	if (n && !copy) return;
	if (n) memcpy(copy, p, (size_t)n * GL_LINE_FLOATS * sizeof(float));
	pthread_mutex_lock(&g->mu);
	free(g->lines);
	g->lines = copy;
	g->nlines = n;
	pthread_mutex_unlock(&g->mu);
}

void gl_quads(glview *g, const float *p, int n) {
	float *copy = n ? malloc((size_t)n * GL_QUAD_FLOATS * sizeof(float)) : NULL;
	if (n && !copy) return;
	if (n) memcpy(copy, p, (size_t)n * GL_QUAD_FLOATS * sizeof(float));
	pthread_mutex_lock(&g->mu);
	free(g->quads);
	g->quads = copy;
	g->nquads = n;
	pthread_mutex_unlock(&g->mu);
}

int gl_read(glview *g, uint8_t *out) {
	if (!g->started) return ERR_ARGS;
	struct timespec until;
	clock_gettime(CLOCK_REALTIME, &until);
	until.tv_sec += 5;
	pthread_mutex_lock(&g->mu);
	g->read_buf = out;
	g->read_want = 1;
	g->read_done = 0;
	g->dirty = 1;
	pthread_cond_broadcast(&g->cv);
	int timed = 0;
	while (!g->read_done && !g->closing && !timed) timed = pthread_cond_timedwait(&g->cv, &g->mu, &until) != 0;
	int ok = g->read_done && g->read_ok;
	g->read_want = 0;
	g->read_buf = NULL;
	pthread_mutex_unlock(&g->mu);
	return ok ? OK : ERR_ARGS;
}

void gl_close(glview *g) {
	if (!g->started) return;
	pthread_mutex_lock(&g->mu);
	g->closing = 1;
	pthread_cond_broadcast(&g->cv);
	pthread_mutex_unlock(&g->mu);
	pthread_join(g->th, NULL);
	for (int i = 0; i < GL_UNITS; i++) free(g->pend[i].pix);
	free(g->src);
	free(g->points);
	free(g->lines);
	free(g->quads);
	pthread_mutex_destroy(&g->mu);
	pthread_cond_destroy(&g->cv);
	g->started = 0;
}
