# Experimental SD-X501F build

This is experimental support for the SD-X501F, which uses a separate hardware profile from the SD-X701F/Blueberry. Firmware and camera tuning files depend on the device's Android installation.

## Build from source

Use Linux or WSL with the Go version listed in `go.mod`, a C toolchain, and the Android NDK version used by CI. Set `ANDROID_NDK_HOME` to the NDK path. This port depends on the companion libcountertop display and camera changes. For local review before the dependency is published, check out both branches in adjacent directories and run `go work init . ../libcountertop` from the LANovo checkout. Once the dependency lands, update the version in `go.mod` and verify with `GOWORK=off` in a clean checkout.

```sh
go test ./internal/board ./internal/hardware/motion ./internal/hardware/display ./internal/hardware/touch ./internal/feature/sensors ./internal/host/takeover ./internal/parts
make BOARD=sd-x501f NDK="$ANDROID_NDK_HOME" build-lanovod
```

The build embeds native helpers and the resulting `bin/lanovod` targets ARMv7/API 27. The optional `COUNTERTOP_FILES_URL` build setting configures a resource host for downloads such as wake-word models.

## Installation and limits

The tested SD-X501F uses Android 8.1 build 5459842 with a compatible camera HAL and tuning files, unlocked AVB, disabled verity and root ADB. The vendor firmware is not provided here. The audio backend depends on Android audio services and an OpenSLES helper; do not stop them during takeover. The current experimental startup waits until Android boot-complete and at least 20 seconds uptime before takeover. The daemon is installed to `/system/bin/lanovod`, while settings, Wi-Fi credentials, wake models and audio helper remain under `/data`.

The X501F launch wrapper sets the Android speaker's `STREAM_MUSIC` policy index to 100/100 before starting LANovo. LANovo's own volume controls then set the listening level. This setting is applied when LANovo starts; it does not change the stored Android volume for standalone Android. The init service also stops a lingering Android boot animation when LANovo is running. Both changes were observed on HUA08MWU after reboot; display and speaker behavior should still be checked on each device.

The Google wake-word resource is not distributed in this repository. Without a configured `COUNTERTOP_FILES_URL`, a fresh install cannot download it automatically. A legitimately obtained, compatible `resources_hotword.pak` can be placed in `/data/misc/lanovo/models/gemini/`; the catalog expects 957360 bytes and SHA-256 `4aeb53fb7b1872bf588ec20ac7bb7cbe727272120607b59014dfe39426ec7b61`. It must be restored after `/data` is erased. On HUA08MWU the model was detected and became ready after a reboot; Home Assistant speech recognition and responses need separate validation.

System installation requires sufficient writable space. A tested device had WebView removed after a verified backup to make space; this repository does not automatically delete WebView or any other Android system application. Back up the target's own partitions, keep a verified EDL recovery path and check all dependencies before changing system. Do not relock AVB on modified partitions. Existing `/data`-only update scripts do not update a daemon running from `/system`; use a verified system-aware updater.

On one SD-X501F test device, screen, touch, physical volume, auto-rotation, camera stills, microphone, speaker and Hey Google were tested. Cold boot, installations on other hardware and the source changes in this clean branch still require independent verification. Home Assistant voice commands additionally require a configured STT/TTS pipeline.
