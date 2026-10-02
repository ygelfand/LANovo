#include <dlfcn.h>
#include <stddef.h>
#include <stdint.h>
#include <string.h>

#define WINDOW_FORMAT 2
#define WINDOW_CONSUMER_USAGE 10
#define FORMAT_RGBA_8888 1
#define FORMAT_IMPLEMENTATION_DEFINED 0x22
#define USAGE_VIDEO_ENCODER 0x10000

int check_permission(const void *perm, int pid, unsigned uid) __asm__("_ZN7android15checkPermissionERKNS_8String16Eij");
int check_permission(const void *perm, int pid, unsigned uid) { return 1; }

int check_calling(const void *perm) __asm__("_ZN7android22checkCallingPermissionERKNS_8String16E");
int check_calling(const void *perm) { return 1; }

int op_check(void *self, int op, int uid, const void *pkg) __asm__("_ZN7android13AppOpsManager7checkOpEiiRKNS_8String16E");
int op_check(void *self, int op, int uid, const void *pkg) { return 0; }

int op_note(void *self, int op, int uid, const void *pkg) __asm__("_ZN7android13AppOpsManager6noteOpEiiRKNS_8String16E");
int op_note(void *self, int op, int uid, const void *pkg) { return 0; }

int op_start(void *self, int op, int uid, const void *pkg) __asm__("_ZN7android13AppOpsManager7startOpEiiRKNS_8String16E");
int op_start(void *self, int op, int uid, const void *pkg) { return 0; }

void op_finish(void *self, int op, int uid, const void *pkg) __asm__("_ZN7android13AppOpsManager8finishOpEiiRKNS_8String16E");
void op_finish(void *self, int op, int uid, const void *pkg) {}

void op_watch(void *self, int op, const void *pkg, const void *cb)
	__asm__("_ZN7android13AppOpsManager17startWatchingModeEiRKNS_8String16ERKNS_2spINS_15IAppOpsCallbackEEE");
void op_watch(void *self, int op, const void *pkg, const void *cb) {}

void op_unwatch(void *self, const void *cb) __asm__("_ZN7android13AppOpsManager16stopWatchingModeERKNS_2spINS_15IAppOpsCallbackEEE");
void op_unwatch(void *self, const void *cb) {}

void sensors_service(void **sret, const void *name, int retry)
	__asm__("_ZN7android10frameworks13sensorservice4V1_014ISensorManager10getServiceERKNSt3__112basic_stringIcNS4_11char_traitsIcEENS4_9allocatorIcEEEEb");
void sensors_service(void **sret, const void *name, int retry) { *sret = NULL; }

void sound_load(void *self) __asm__("_ZN7android13CameraService9loadSoundEv");
void sound_load(void *self) {}

void sound_release(void *self) __asm__("_ZN7android13CameraService12releaseSoundEv");
void sound_release(void *self) {}

void sound_play(void *self, int kind) __asm__("_ZN7android13CameraService9playSoundENS0_10sound_kindE");
void sound_play(void *self, int kind) {}

int proc_scores(void *self, size_t n, int32_t *pids, int32_t *states, int32_t *scores)
	__asm__("_ZN7android18ProcessInfoService26getProcessStatesScoresImplEjPiS1_S1_");
int proc_scores(void *self, size_t n, int32_t *pids, int32_t *states, int32_t *scores) {
	for (size_t i = 0; i < n; i++) {
		states[i] = 2;
		scores[i] = 0;
	}
	return 0;
}

int request_priority(int pid, int tid, int prio, int app, int async) __asm__("_ZN7android15requestPriorityEiiibb");
int request_priority(int pid, int tid, int prio, int app, int async) { return 0; }

typedef int (*query_fn)(const void *self, int what, int *value);

int surface_query(const void *self, int what, int *value) __asm__("_ZNK7android7Surface5queryEiPi");
int surface_query(const void *self, int what, int *value) {
	static query_fn real;
	if (!real) real = (query_fn)dlsym(RTLD_NEXT, "_ZNK7android7Surface5queryEiPi");
	int r = real(self, what, value);
	if (r != 0 || what != WINDOW_FORMAT || *value != FORMAT_RGBA_8888) return r;
	int usage = 0;
	if (real(self, WINDOW_CONSUMER_USAGE, &usage) == 0 && (usage & USAGE_VIDEO_ENCODER)) *value = FORMAT_IMPLEMENTATION_DEFINED;
	return r;
}

typedef unsigned (*sleep_fn)(unsigned secs);

static int from_binder(const void *at) {
	Dl_info info;
	if (!dladdr(at, &info) || !info.dli_fname) return 0;
	size_t n = strlen(info.dli_fname), want = sizeof "/libbinder.so" - 1;
	return n >= want && !strcmp(info.dli_fname + n - want, "/libbinder.so");
}

unsigned sleep(unsigned secs) {
	static sleep_fn real;
	if (from_binder(__builtin_return_address(0))) return 0;
	if (!real) real = (sleep_fn)dlsym(RTLD_NEXT, "sleep");
	return real(secs);
}
