#include <pthread.h>
#include <stdint.h>

#include <android/native_window.h>

#define UI_VERTEX_FLOATS 9
#define UI_QUAD_FLOATS (UI_VERTEX_FLOATS * 4)
#define UI_RUN_WORDS 40
#define UI_PROGRAMS 8
#define UI_TEXTURES 256

typedef struct {
	uint32_t id;
	int w, h, x, y, rw, rh;
	uint8_t *pix;
} ui_upload;

typedef struct {
	ANativeWindow *win;
	int w, h;
	pthread_t th;
	int started;
	pthread_mutex_t mu;
	pthread_cond_t cv;
	int closing, dirty, drawn;
	char *vs[UI_PROGRAMS], *fs[UI_PROGRAMS];
	ui_upload *ups;
	int nups, upcap;
	float clear[4];
	int rot;
	float *quads;
	int nquads, quadcap;
	uint32_t *runs;
	int nruns, runcap;
	uint32_t frame, done;
	uint32_t submit_us, finish_us;
	uint8_t *read_buf;
	int read_want, read_done, read_ok;
} uiview;

int ui_open(uiview *u, ANativeWindow *win, int w, int h);
void ui_program(uiview *u, int slot, const char *vs, int vslen, const char *fs, int fslen);
void ui_texture(uiview *u, uint32_t id, int w, int h, int x, int y, int rw, int rh, const uint8_t *pix);
uint32_t ui_frame(uiview *u, int rot, const float clear[4], const float *quads, int nquads, const uint32_t *runs, int nruns);
void ui_wait(uiview *u, uint32_t frame, uint32_t *submit_us, uint32_t *finish_us);
int ui_read(uiview *u, uint8_t *out);
void ui_close(uiview *u);
