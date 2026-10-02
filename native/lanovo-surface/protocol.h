#define SOCKET_NAME "lanovo-surface"
#define SOCKET_PATH "/dev/socket/" SOCKET_NAME

#define MAGIC 0x53564e4c
#define VERSION 4

#define OP_HELLO 1
#define OP_CREATE 2
#define OP_FRAME 3
#define OP_SCENE 4
#define OP_DESTROY 5
#define OP_VIDEO_OPEN 6
#define OP_VIDEO_SAMPLE 7
#define OP_VIDEO_CLOCK 8
#define OP_VIDEO_PLACE 9
#define OP_VIDEO_CLOSE 10
#define OP_VIDEO_FLUSH 11
#define OP_GL_OPEN 12
#define OP_GL_TEXTURE 13
#define OP_GL_PROGRAM 14
#define OP_GL_VALUES 15
#define OP_GL_POINTS 16
#define OP_GL_LINES 17
#define OP_GL_QUADS 18
#define OP_GL_OFFSCREEN 19
#define OP_GL_READ 20
#define OP_DRM_OPEN 21
#define OP_DRM_REQUEST 22
#define OP_DRM_PROVIDE 23
#define OP_DRM_CLOSE 24
#define OP_VIDEO_CRYPT 25
#define OP_AUDIO_OPEN 26
#define OP_AUDIO_SAMPLE 27
#define OP_AUDIO_CLOSE 28
#define OP_SCREEN_READ 29
#define OP_UI_OPEN 30
#define OP_UI_PROGRAM 31
#define OP_UI_TEXTURE 32
#define OP_UI_FRAME 33
#define OP_UI_READ 34

#define CRYPT_CLEAR 0
#define CRYPT_CENC 1
#define CRYPT_CBCS 2

#define CRYPT_WORDS 12

#define CODEC_VP9 1
#define CODEC_AVC 2
#define CODEC_HEVC 3
#define CODEC_VP8 4

#define SAMPLE_KEY 1
#define SAMPLE_END 2

#define ERR_CODEC 5
#define ERR_AGAIN 6
#define ERR_SHADER 7
#define ERR_DRM 8

#define FLAG_OPAQUE 1
#define FLAG_SECURE 2

#define OK 0
#define ERR_ARGS 1
#define ERR_FULL 2
#define ERR_SURFACE 3
#define ERR_MEMORY 4
