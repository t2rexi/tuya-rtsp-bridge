# Changelog

## 1.2.6

- HA add-on: bust Docker git-clone cache (stale 1.2.4 image after “update”)
- Discover cameras via login `extras.homeId` when homeList is empty
- Do not print session JSON (`sid`) in the web log
- Windows preview still uses SD; a black tile with VLC 3 + HEVC is not a dead RTSP stream

## 1.2.5

- Fix empty camera list after login (#1): more Tuya categories (doorbells, `sp_*`, p2p), keep devices if jarvis/config fails, show `homes=`/`skipped=` in the UI
- HA add-on README: troubleshooting for Session-active / 0 cameras

## 1.2.4

- Fix Windows QR display: Canvas + fixed 320×320 NEAREST (no Tk Label slit) — #2
- Windows DPI awareness; API reconnects if backend was down (WinError 10061)
- Home Assistant OS add-on skeleton (`homeassistant/tuya_rtsp_bridge`) — #1
- Cloud PTZ fallback + protect password auto-relogin (local builds)
- Docs parity: 17 locales each have faq / getting-started / why / windows / api / docker / nvr

## 1.2.3

- Honest licenses: bundled ffmpeg is GPL-3 (Gyan essentials), VLC zip is GPL-2 / libVLC LGPL
- Short legal notes: [docs/legal.md](docs/legal.md), [docs/rechtliches.md](docs/rechtliches.md)
- Same app as 1.2.2

## 1.2.2

- App UI and docs in 18 languages (added Japanese, Korean, Hebrew, Yiddish)
- Windows Setup wizard: Japanese, Korean, Hebrew (Yiddish stays in the app menu)
- Project mark on the installer and desktop shortcut
- Docs no longer ask Windows users to install Python / VLC / ffmpeg

## 1.2.1

- Same bundled runtime as 1.2.0
- App icon on Setup.exe and shortcuts

## 1.2.0

- Foolproof Setup: private CPython, VLC, ffmpeg, RTSP engine
- No system Python, no extra installers

## 1.1.0

- Arch packaging, more languages, no Windows EXE on that tag

## 1.0.0

- First public release (small Setup, no bundled runtime)
