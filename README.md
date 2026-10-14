# LANovo

<a href="https://github.com/ygelfand/LANovo/stargazers"><img src="https://img.shields.io/github/stars/ygelfand/LANovo?style=for-the-badge&label=Stars&color=d6a102" alt="Stars"></a>
<a href="https://github.com/ygelfand/LANovo/releases"><img src="https://img.shields.io/github/downloads/ygelfand/LANovo/total?style=for-the-badge&label=Downloads&color=e8604c" alt="Downloads"></a>
<a href="https://github.com/ygelfand/LANovo/releases/latest"><img src="https://shields.io/github/v/release/ygelfand/LANovo?style=for-the-badge&color=5da3a6" alt="Version"></a>
<a href="https://buymeacoffee.com/ygelfand"><img src="https://img.shields.io/badge/Buy_me_a_coffee-ffdd00?style=for-the-badge&logo=buymeacoffee&logoColor=000" alt="Buy me a coffee"></a>
<a href="https://discord.gg/efWnv89pg2"><img src="https://img.shields.io/badge/Join_the_community-5865f2?style=for-the-badge&logo=discord&logoColor=fff" alt="Join the community on Discord"></a>
<a href="https://www.lanovo.link"><img src="https://img.shields.io/badge/Docs-www.lanovo.link-2f8cff?style=for-the-badge" alt="Documentation"></a>

Turn a Lenovo Smart Display into a local Home Assistant display, speaker and voice assistant.

A pure-Go replacement for what Lenovo and Google shipped. It speaks the ESPHome native API, so Home
Assistant discovers it the way it discovers any ESPHome device.

<p align="center">
  <img src="assets/readme/visual-wall.webp" alt="Six of LANovo's visuals: Synthwave, LCARS, Classic VU, Aurora, a Pip-Boy, and a koi pond" width="720">
</p>

## Devices

| Display | Codename | Status |
| --- | --- | --- |
| Lenovo Smart Display 10 | blueberry | Supported, portrait and landscape |
| Lenovo Smart Display 8 | amber | Portrait and landscape, untested |
| Lenovo Smart Display 7 | ivy | Supported, landscape |
| JBL Link View | opal | Supported, landscape |
| Lenovo ThinkSmart View | penghu | Supported, portrait and landscape |

## Features

### Visuals

33 audio visualizers, all GPU shaders at the screen's full resolution. Each one reacts to the
microphone, to what's playing, or both. Run one full screen or two side by side, on the screen
saver or for each voice assistant while it listens and answers. LCARS, HAL 9000,
a lava lamp, rain on glass, light cycles, four Pip-Boys, studio VU meters and more.

### Native dashboard

A clock in one of eight faces, the weather, and tabs for your Home Assistant lights and switches,
picked by area or label.

<p align="center">
  <img src="assets/readme/faces.webp" alt="Eight clock faces: Plain, Stack, Cards, Analog, Segments, Words, Overlap and Analog seconds" width="260">
  <img src="assets/readme/home-lights.webp" alt="The Lights tab, with room cards" width="360">
</p>

### Tessera dashboards

LANovo speaks the [Tessera](https://github.com/MaxGramser/homeassistant_espscreen) screen protocol.
Lay out pages in Tessera's editor in Home Assistant and the display draws them. Most of Tessera's
tiles and features are supported.

<p align="center">
  <img src="assets/readme/tessera-styles.webp" alt="One Tessera page in each of the nine styles: Standard, Material You, Glass, E-ink, Metro, Terminal, Hi-fi, Neon and Skeuo" width="360">
</p>

### Styles and themes

Nine styles (Standard, Material You, Glass, E-ink, Metro, Terminal, Hi-fi, Neon, Skeuo) and eighteen
color themes, applied to everything on the display, Tessera pages included.

### Rotation

The Smart Display 10, Smart Display 8 and ThinkSmart View rotate automatically. The
dashboards, settings, camera and everything else turn with them.

<p align="center">
  <img src="assets/readme/rotate.webp" alt="The same dashboard in landscape and in portrait" width="640">
</p>

### Voice

On-device wake words with [microWakeWord](https://github.com/kahrendt/microWakeWord) and
[openWakeWord](https://github.com/dscripka/openWakeWord). Two assistants, each with its own wake
word, Assist pipeline and visuals. "Stop" works without a wake word.

### Intercom

Audio and video calls between LANovo displays and EchoLocal devices on your network, with
auto-answer for the devices you choose.

<p align="center">
  <img src="assets/readme/intercom.webp" alt="The Intercom directory listing displays and Echos on the network" width="420">
</p>

### Timers and messages

Six timer styles. Messages from Home Assistant in four tones and four styles, each with its own
chime, including custom sounds.

<p align="center">
  <img src="assets/readme/timers.webp" alt="A timer in six styles" width="260">
  <img src="assets/readme/alerts.webp" alt="A message as a banner, a card, a toast and full screen" width="260">
</p>

### Audio

- A Home Assistant `media_player` for announcements, TTS and radio
- A Sendspin player for Music Assistant
- A Bluetooth speaker
- A Chromecast receiver for YouTube (with SponsorBlock), YouTube Music, YouTube TV, Prime Video and
  the default media receiver (experimental)

### Camera

An RTSP server on port 8554 with `main` and `sub` streams for Frigate, Blue Iris or similar, ONVIF
discovery, and a snapshot camera in Home Assistant. The physical shutter stops the camera.

### Also

- Bluetooth proxy for Home Assistant
- Proximity wake and adaptive brightness
- Night mode: dim, dim to a clock, or screen off when the room is dark
- Immich wallpaper
- Settings on the display or from Home Assistant, and config sync between displays
- Updates from Home Assistant or from the display, stable or development channel
- Apps: Intercom, Mirror, Shopping list, Calendar, Radar, more coming

## Installing

You need an unlocked display connected over USB. Follow the XDA guide for the
[Smart Display 8/10 and JBL Link View](https://xdaforums.com/t/lenovo-smart-display-8-10-amber-blueberry-avb-bootloader-unlock-firmware.4472049/)
or the
[Smart Display 7](https://xdaforums.com/t/cd-17302f-lenovo-smart-display-7-ivy-avb-bootloader-unlock-firmware.4472041/)
and flash the **Android 8.1 Debug Firmware**. For the ThinkSmart View, ask on
[Discord](https://discord.gg/efWnv89pg2).

Then download `lanovo` from the [releases page](https://github.com/ygelfand/LANovo/releases):

```sh
lanovo install --name kitchen
```

It checks the display, installs LANovo, and asks for your Wi‑Fi if the display isn't on a network
yet. It then shows up in Home Assistant, with a code on screen to pair it.

<p align="center">
  <img src="assets/readme/sd10-onboard.jpg" alt="A Smart Display 10 showing a code to add it to Home Assistant" height="260">
</p>

The full guide is at [www.lanovo.link](https://www.lanovo.link).

## Building it yourself

```sh
make build             # lanovo, lanovod and its native helpers
make dist              # lanovod, then lanovo carrying it
make install-lanovod   # build, install, and restart it on a connected display
```

## How it fits together

- **lanovod** runs on the display: the screen, the audio, the camera, the radios, the wake word
  engines, the conversation, and an ESPHome native API server. It is one static Go binary with no
  cgo.
- **lanovo** is the host CLI: checking, installing, Wi‑Fi and pairing.
- **[libcountertop](https://github.com/ygelfand/libcountertop)** holds the features LANovo shares
  with [EchoLocal](https://github.com/ygelfand/echolocal).
- **[go-esphome-device](https://github.com/ygelfand/go-esphome-device)** implements the device half of
  the ESPHome protocol, including the voice satellite and Bluetooth proxy.

## License

AGPL-3.0. See [LICENSE](LICENSE).
