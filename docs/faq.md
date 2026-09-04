# FAQ

### The camera list is empty after login

1. Wrong region. “Western Europe” in the German app is **EU** (`protect-eu`), not WE. Try the other cluster in the same continent.
2. Click **Refresh cameras** (or Create QR again). The UI now shows a short note (`homes=… skipped=category:n`) instead of a blank “No cameras.”
3. Doorbells and some battery cams are not category `sp`. 1.2.5+ treats `videolock`, `sp_*`, p2p, and cloud-storage devices as cameras too.
4. If `homes=0`, the session cookie did not reach `homeList` — Sign out, Create QR, confirm in Smart Life.

### The QR never finishes

Keep the window open and **confirm** in the phone. A pending poll (`result: true` without a user id) is normal.

### The QR is tiny / a horizontal slit / unscannable (Windows)

Fixed in **1.2.4+**: the GUI draws the QR on a fixed **320×320** canvas (NEAREST pixels), not a Tk Label that collapses to character cells. Update the app / rebuild the Setup. If the box still says “No QR”, click **Create QR** and wait for the image — empty placeholder is normal before that.

### Backend connection refused (WinError 10061)

The desktop UI now starts the local API (`:8787`) automatically if it was down. Click **Create QR** again, or use **Restart UI / API**.

### VLC is black / the preview is a slit

VLC 3 often fails on HEVC over RTSP. That does **not** mean the stream is dead. Use Agent DVR, Frigate, or ffplay on `rtsp://127.0.0.1:8554/<Name>/hd`. The desktop preview uses **SD**. On Linux the GUI uses an ffmpeg MJPEG pipe instead of embedded VLC.

### I expected 60 fps

Many Tuya IPC models emit about **10 fps** in the HD HEVC stream. This bridge does not invent frames.

### Is this ONVIF?

No. Stock Tuya firmware does not speak ONVIF. This project is RTSP-only.

### Is video leaving my house?

Signaling (login, WebRTC handshake) goes to Tuya. When you view through this PC, the video medium is typically camera → this PC on the LAN. A phone on mobile data is a **second** viewer and uses the cloud path.

### Can I use go2rtc `tuya://` instead?

That needs a Tuya Smart **email/password**, not a Smart Life QR. Different login.

### Cloud PTZ when I am not on the LAN?

LAN PTZ (TCP **6668**) is preferred when the camera is reachable. Off-site, the bridge can fall back to the reverse mobile API (`tuya.m.device.dp.publish`) using your **Smart Life / Tuya Smart email + password** once (stored only in local `cloud_auth.json`, mode 600 — no Tuya IoT Platform developer keys). Set via `POST /api/cloud/auth` or the first cloud PTZ attempt after credentials exist.

### HLS / VLC HTTP?

Optional and expensive (x264 transcode). Off by default. Agent/Frigate should use RTSP `/hd`.

### Where is my login stored?

`%APPDATA%\TuyaRtspBridge\` (Windows) or `~/.local/share/tuya-rtsp-bridge/` (Linux) — cookies, camera list, optional cloud password. Do not put that folder in git or in a screenshot.

### Home Assistant add-on?

Yes — Supervisor add-on under [`homeassistant/tuya_rtsp_bridge/`](../homeassistant/tuya_rtsp_bridge/). Needs **host network**. Plain Docker on the HA host is still fine: [docker.md](docker.md).

### Linux / macOS?

`./launch.sh` from a clone. Arch package: [docs/arch-linux.md](arch-linux.md). Data lives in `~/.local/share/tuya-rtsp-bridge/`.
