#include <pthread.h>
#include <stdint.h>

#include <android/native_window.h>

#define GL_UNITS 4
#define GL_VALUES 128

#define GL_WITH_LIGHT 1
#define GL_WITH_PRE 2
#define GL_WITH_FEED 4
#define GL_FEED_HALF 8
#define GL_WITH_SPLAT 16
#define GL_SPLAT_HALF 32
#define GL_WITH_LINES 64
#define GL_FEED_FLOAT 128

#define GL_POINT_FLOATS 8
#define GL_POINT_FEED 1
#define GL_POINT_ROUND 2
#define GL_POINT_LIGHT 4
#define GL_POINT_SPLAT 8
#define GL_POINT_DISC 16
#define GL_POINT_OVER 32

#define GL_LINE_FLOATS 9
#define GL_QUAD_FLOATS 12
#define GL_PRE_ROWS 8

typedef struct {
	int w, h;
	uint8_t *pix;
} gl_tex;

typedef struct {
	float amount, radius;
	int passes;
} gl_glow;

typedef struct {
	ANativeWindow *win;
	int w, h;
	pthread_t th;
	int started;
	pthread_mutex_t mu;
	pthread_cond_t cv;
	int closing, dirty;
	gl_tex pend[GL_UNITS];
	char *src;
	int src_flags;
	int compiled, compile_done;
	float u[GL_VALUES];
	int nu;
	gl_glow glow;
	float *points;
	int npoints;
	float *lines;
	int nlines;
	float *quads;
	int nquads;
	uint8_t *read_buf;
	int read_want, read_done, read_ok;
} glview;

int gl_open(glview *g, ANativeWindow *win, int w, int h);
void gl_texture(glview *g, int unit, int w, int h, const uint8_t *pix);
int gl_program(glview *g, const char *src, int len, int flags);
void gl_values(glview *g, const float *u, int n, gl_glow glow);
void gl_points(glview *g, const float *p, int n);
void gl_lines(glview *g, const float *p, int n);
void gl_quads(glview *g, const float *p, int n);
int gl_read(glview *g, uint8_t *out);
void gl_close(glview *g);
