#include <android/native_window.h>

typedef struct cam2 cam2;

int cam2_present(void);
cam2 *cam2_open(ANativeWindow **outs, int n, const char *knobs);
int cam2_apply(cam2 *c, const char *knobs);
void cam2_close(cam2 *c);
