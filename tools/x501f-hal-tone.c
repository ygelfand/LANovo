#include <SLES/OpenSLES.h>
#include <SLES/OpenSLES_Android.h>
#include <math.h>
#include <stdint.h>
#include <string.h>
#include <stdio.h>
#include <unistd.h>

#define CHECK(call) do { SLresult r = (call); if (r != SL_RESULT_SUCCESS) { \
    fprintf(stderr, "%s: %u\n", #call, (unsigned)r); return 1; } } while (0)

int main(int argc, char **argv) {
    int pipe_mode = argc == 2 && strcmp(argv[1], "--stdin") == 0;
    if (argc != 1 && !pipe_mode) {
        fprintf(stderr, "usage: %s [--stdin]\n", argv[0]);
        return 2;
    }
    SLObjectItf engine_object, mix, player;
    SLEngineItf engine;
    SLPlayItf play;
    SLAndroidSimpleBufferQueueItf queue;
    SLAndroidConfigurationItf config;
    CHECK(slCreateEngine(&engine_object, 0, NULL, 0, NULL, NULL));
    CHECK((*engine_object)->Realize(engine_object, SL_BOOLEAN_FALSE));
    CHECK((*engine_object)->GetInterface(engine_object, SL_IID_ENGINE, &engine));
    CHECK((*engine)->CreateOutputMix(engine, &mix, 0, NULL, NULL));
    CHECK((*mix)->Realize(mix, SL_BOOLEAN_FALSE));

    SLDataLocator_AndroidSimpleBufferQueue locator = {SL_DATALOCATOR_ANDROIDSIMPLEBUFFERQUEUE, 2};
    SLDataFormat_PCM pcm = {SL_DATAFORMAT_PCM, 2, SL_SAMPLINGRATE_48,
        SL_PCMSAMPLEFORMAT_FIXED_16, SL_PCMSAMPLEFORMAT_FIXED_16,
        SL_SPEAKER_FRONT_LEFT | SL_SPEAKER_FRONT_RIGHT, SL_BYTEORDER_LITTLEENDIAN};
    SLDataSource source = {&locator, &pcm};
    SLDataLocator_OutputMix output = {SL_DATALOCATOR_OUTPUTMIX, mix};
    SLDataSink sink = {&output, NULL};
    SLInterfaceID ids[] = {SL_IID_ANDROIDSIMPLEBUFFERQUEUE, SL_IID_ANDROIDCONFIGURATION};
    SLboolean required[] = {SL_BOOLEAN_TRUE, SL_BOOLEAN_TRUE};
    CHECK((*engine)->CreateAudioPlayer(engine, &player, &source, &sink, 2, ids, required));
    CHECK((*player)->GetInterface(player, SL_IID_ANDROIDCONFIGURATION, &config));
    SLint32 stream = SL_ANDROID_STREAM_MEDIA;
    CHECK((*config)->SetConfiguration(config, SL_ANDROID_KEY_STREAM_TYPE, &stream, sizeof stream));
    CHECK((*player)->Realize(player, SL_BOOLEAN_FALSE));
    CHECK((*player)->GetInterface(player, SL_IID_PLAY, &play));
    CHECK((*player)->GetInterface(player, SL_IID_ANDROIDSIMPLEBUFFERQUEUE, &queue));

    if (pipe_mode) {
        // Keep two buffers queued while the next 20 ms block is read.
        unsigned char buffers[2][960 * 4];
        CHECK((*play)->SetPlayState(play, SL_PLAYSTATE_PLAYING));
        unsigned next = 0;
        for (;;) {
            SLAndroidSimpleBufferQueueState state;
            do {
                CHECK((*queue)->GetState(queue, &state));
                if (state.count >= 2) usleep(2000);
            } while (state.count >= 2);
            size_t n = fread(buffers[next], 1, sizeof buffers[next], stdin);
            if (n % 4 != 0 || ferror(stdin)) return 1;
            if (!n) break;
            CHECK((*queue)->Enqueue(queue, buffers[next], n));
            next ^= 1;
        }
        SLAndroidSimpleBufferQueueState state;
        for (int i = 0; i < 500; ++i) {
            CHECK((*queue)->GetState(queue, &state));
            if (!state.count) break;
            usleep(2000);
        }
        (*player)->Destroy(player);
        (*mix)->Destroy(mix);
        (*engine_object)->Destroy(engine_object);
        return state.count != 0;
    }

    static int16_t samples[48000 * 5 * 2];
    for (int i = 0; i < 48000 * 5; ++i)
        samples[2*i] = samples[2*i+1] = (int16_t)(16000 * sin(i * 440.0 * 6.283185307 / 48000));
    CHECK((*queue)->Enqueue(queue, samples, sizeof samples));
    CHECK((*play)->SetPlayState(play, SL_PLAYSTATE_PLAYING));
    puts("Android AudioTrack playback started (440 Hz, 5 seconds)");
    fflush(stdout);
    sleep(6);
    SLAndroidSimpleBufferQueueState state;
    CHECK((*queue)->GetState(queue, &state));
    printf("consumed buffers: %u, pending: %u\n", (unsigned)state.index, (unsigned)state.count);
    (*player)->Destroy(player);
    (*mix)->Destroy(mix);
    (*engine_object)->Destroy(engine_object);
    return state.count != 0;
}
