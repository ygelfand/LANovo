#define SOCKET_NAME "lanovo-camera"
#define SOCKET_PATH "/dev/socket/" SOCKET_NAME

#define MAGIC 0x4d434e4c
#define VERSION 5

#define OPEN_WORDS 11
#define NO_TURN 0xffffffffu
#define TURN_MIRROR 0x100
#define REPLY_WORDS 4
#define FRAME_WORDS 4

#define FRAME_KEY 1
#define FRAME_CONFIG 2
#define FRAME_SUB 4
#define FRAME_STILL 8

#define ASK_STILL 'S'
#define ASK_PARAMS 'P'
#define MAX_PARAMS 4096

#define OK 0
#define ERR_ARGS 1
#define ERR_CAMERA 2
#define ERR_ENCODER 3
#define ERR_VERSION 4
