# Experimental SD-X501F build

This is experimental support for the SD-X501F, which uses a separate hardware profile from the SD-X701F/Blueberry. Firmware and camera tuning files depend on the device's Android installation.

## Build from source

Use Linux or WSL with the Go version listed in `go.mod`, a C toolchain, and the Android NDK version used by CI. Set `ANDROID_NDK_HOME` to the NDK path. This port depends on the companion libcountertop display and camera changes. For local review before the dependency is published, check out both branches in adjacent directories and run `go work init . ../libcountertop` from the LANovo checkout. Once the dependency lands, update the version in `go.mod` and verify with `GOWORK=off` in a clean checkout.

On WSL, clone both repositories from Linux with `git -c core.autocrlf=false clone ...` so the AirPlay dependency keeps its original checksum files.

```sh
go test ./internal/board ./internal/hardware/motion ./internal/hardware/display ./internal/hardware/touch ./internal/feature/sensors ./internal/host/takeover ./internal/parts
make BOARD=sd-x501f NDK="$ANDROID_NDK_HOME" dist
```

The device build embeds native helpers and targets ARMv7 with API 27 native components. The `dist` target embeds that daemon and the board-specific install helpers into the host CLI in `bin/lanovo`. Run `./bin/lanovo install --name kitchen` from the host after confirming it sees the target device. The optional `COUNTERTOP_FILES_URL` build setting configures a resource host for downloads such as wake-word models.

## Installation and limits

The tested SD-X501F uses Android 8.1 build 5459842 with a compatible camera HAL and tuning files, unlocked AVB, disabled verity and root ADB. The vendor firmware is not provided here. The audio backend depends on Android audio services and an OpenSLES helper; do not stop them during takeover. The current experimental startup waits until Android boot-complete and at least 20 seconds uptime before takeover. The daemon is installed to `/system/bin/lanovod`, while settings, Wi-Fi credentials, wake models and audio helper remain under `/data`.

The X501F launch wrapper sets the Android speaker's `STREAM_MUSIC` policy index to 100/100 before starting LANovo. LANovo's own volume controls then set the listening level. This setting is applied when LANovo starts; it does not change the stored Android volume for standalone Android. The init service also stops a lingering Android boot animation when LANovo is running. Both changes were observed on HUA08MWU after reboot; display and speaker behavior should still be checked on each device.

The Google wake-word resource is not distributed in this repository. Without a configured `COUNTERTOP_FILES_URL`, a fresh install cannot download it automatically. A legitimately obtained, compatible `resources_hotword.pak` can be placed in `/data/misc/lanovo/models/gemini/`; the catalog expects 957360 bytes and SHA-256 `4aeb53fb7b1872bf588ec20ac7bb7cbe727272120607b59014dfe39426ec7b61`. It must be restored after `/data` is erased. On HUA08MWU the model was detected and became ready after a reboot; Home Assistant speech recognition and responses need separate validation.

System installation requires enough free space to stage a replacement daemon next to the existing one. On a retry, the installer reuses an executable daemon if its SHA-256 matches the install payload. Back up the target's own partitions and keep a verified recovery path. The installer leaves the boot images and Android applications in place. Use the SD-X501F `lanovo install` build for updates. LANovo OTA is disabled on this profile.

The first install saves the original `/init.msm8x53.rc` as `/init.msm8x53.rc.lanovo.bak` before adding its LANovo import. If `/default.prop` needs to change, it also saves `/default.prop.lanovo.bak`. When recovering Android, mount the affected slot's system image read-write from recovery, verify the backups against that slot's stock files, and rename each verified backup to its original path. Restore the mount to read-only and reboot. If either required backup is missing or unverifiable, restore the matching stock system image from the target's own backup. Only restore bootloader/AVB lock after restoring the complete signed stock firmware.

If installation is interrupted, the rename protects each file independently. A retry with the same installer can finish writing the remaining files. If LANovo took over and the target is no longer accessible, use the recovery path above.

If Android reports the system mount is busy after the installer syncs its writes, the installer retries the read-only remount. If the mount is still busy, it saves the device name and reboots to complete the install, then verifies the system mount is read-only. With `--no-reboot`, reboot manually before continuing setup.

On one SD-X501F test device, screen, touch, physical volume, auto-rotation, camera stills, microphone, speaker and Hey Google were tested. Cold boot, installations on other hardware and the source changes in this clean branch still require independent verification. Home Assistant voice commands additionally require a configured STT/TTS pipeline.
