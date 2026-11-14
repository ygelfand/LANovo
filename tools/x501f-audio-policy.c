#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>

typedef int (*init_volume_fn)(int, int, int);
typedef int (*set_volume_fn)(int, int, unsigned int);

int main(int argc, char **argv) {
    if (argc != 2) return 2;
    char *end;
    long index = strtol(argv[1], &end, 10);
    if (*end || index < 0 || index > 100) return 2;
    void *lib = dlopen("libaudioclient.so", RTLD_NOW);
    if (!lib) { fprintf(stderr, "dlopen: %s\n", dlerror()); return 1; }
    init_volume_fn init = (init_volume_fn)dlsym(lib, "_ZN7android11AudioSystem16initStreamVolumeE19audio_stream_type_tii");
    set_volume_fn set = (set_volume_fn)dlsym(lib, "_ZN7android11AudioSystem20setStreamVolumeIndexE19audio_stream_type_tij");
    if (!init || !set) { fprintf(stderr, "AudioSystem functions missing\n"); return 1; }
    int a = init(3, 0, 100);
    int b = set(3, (int)index, 2);
    printf("init=%d set=%d music=%ld device=speaker\n", a, b, index);
    return a || b ? 1 : 0;
}
