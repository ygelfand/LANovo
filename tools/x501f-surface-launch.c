#include <stdio.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/un.h>
#include <unistd.h>

/* Give the existing helper a private socket, as Android init normally does. */
int main(int argc, char **argv) {
    struct sockaddr_un address = {.sun_family = AF_UNIX};
    if (argc != 3 || argv[2][0] != '@' || !argv[2][1] ||
        strlen(argv[2]) > sizeof address.sun_path) {
        fprintf(stderr, "usage: %s HELPER @ABSTRACT_SOCKET\n", argv[0]);
        return 2;
    }
    size_t length = strlen(argv[2]);
    memcpy(address.sun_path + 1, argv[2] + 1, length - 1);
    int fd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (fd < 0 || bind(fd, (struct sockaddr *)&address,
        offsetof(struct sockaddr_un, sun_path) + length) < 0) {
        perror("surface trial socket");
        return 1;
    }
    char value[24];
    snprintf(value, sizeof value, "%d", fd);
    if (setenv("ANDROID_SOCKET_lanovo-surface", value, 1) == 0)
        execl(argv[1], argv[1], (char *)NULL);
    perror("starting surface helper");
    return 1;
}
