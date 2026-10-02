#include <android/native_window.h>

typedef struct turner turner;

turner *turn_open(ANativeWindow *main, int mw, int mh, ANativeWindow *sub, int sw, int sh, int cw, int ch, int quarters,
	ANativeWindow **camera);
int turn_frame(turner *t, unsigned char **still);
void turn_close(turner *t);
