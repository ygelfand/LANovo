# Experimental SD-X501F build

This branch builds on upstream LANovo 0.0.7 (7397791) and is experimental. SD-X501F is a separate hardware profile from SD-X701F/Blueberry. Do not flash images from another device or use this on an unverified model. Firmware, camera tuning blobs, private device backups, keys, binary artifacts and third-party APKs are not included.

## Build from source

Linux or WSL is recommended. Install Go 1.27, `patch`, a C toolchain and Android NDK r27d; set `ANDROID_NDK_HOME` to the NDK path. Network access is needed to download Go modules. From the repository root:

```sh
sh tools/prepare-countertop.sh
go test ./internal/board ./internal/hardware/motion ./internal/hardware/display ./internal/hardware/touch ./internal/feature/sensors ./internal/host/takeover ./internal/parts
make BOARD=sd-x501f NDK="$ANDROID_NDK_HOME" build-lanovod
```

The preparation script downloads the exact upstream libcountertop module pinned in go.mod and applies the five-file patch locally. The generated `third_party/libcountertop` is ignored by Git; review the patch separately before proposing it upstream. Do not run the script on a pre-existing directory; move it first if regeneration is required. The build embeds native helpers and the resulting `bin/lanovod` targets ARMv7/API 27. Upstream's optional `COUNTERTOP_FILES_URL` build setting is needed for downloads such as wake-word models; without an authorized resource host, entries may appear but fail to download.

## Installation and limits

The tested SD-X501F uses Android 8.1 build 5459842 with a compatible camera HAL and tuning files, unlocked AVB, disabled verity and root ADB. The vendor firmware is not provided here. The audio backend depends on Android audio services and an OpenSLES helper; do not stop them during takeover. The current experimental startup waits until Android boot-complete and at least 20 seconds uptime before takeover. The daemon is installed to `/system/bin/lanovod`, while settings, Wi-Fi credentials, wake models and audio helper remain under `/data`.

The X501F launch wrapper sets the Android speaker's `STREAM_MUSIC` policy index to 100/100 before starting LANovo. LANovo's own volume controls then set the listening level. This setting is applied when LANovo starts; it does not change the stored Android volume for standalone Android. The init service also stops a lingering Android boot animation when LANovo is running. Both changes were observed on HUA08MWU after reboot; display and speaker behavior should still be checked on each device.

The Google wake-word resource is not distributed in this repository. Without a configured `COUNTERTOP_FILES_URL`, a fresh install cannot download it automatically. A legitimately obtained, compatible `resources_hotword.pak` can be placed in `/data/misc/lanovo/models/gemini/`; the catalog expects 957360 bytes and SHA-256 `4aeb53fb7b1872bf588ec20ac7bb7cbe727272120607b59014dfe39426ec7b61`. It must be restored after `/data` is erased. On HUA08MWU the model was detected and became ready after a reboot; Home Assistant speech recognition and responses need separate validation.

System installation requires sufficient writable space. A tested device had WebView removed after a verified backup to make space; this repository does not automatically delete WebView or any other Android system application. Back up the target's own partitions, keep a verified EDL recovery path and check all dependencies before changing system. Do not relock AVB on modified partitions. Existing `/data`-only update scripts do not update a daemon running from `/system`; use a verified system-aware updater.

On one SD-X501F test device, screen, touch, physical volume, auto-rotation, camera stills, microphone, speaker and Hey Google were tested. Cold boot, installations on other hardware and the source changes in this clean branch still require independent verification. Home Assistant voice commands additionally require a configured STT/TTS pipeline.

## Contribution plan

Review the X501F-specific diff against upstream, extract generic changes into separate upstreamable commits, and send the libcountertop changes to that project first or coordinate a published dependency revision. Replace the local module override with that revision before a conventional upstream PR. Do not publish private backups, device identity files, model packs or credentials. Do not submit this experimental branch as a finished upstream PR until the clean source build, installation, rollback and multiple-device tests pass.
