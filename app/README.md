# Port8080 host (early release)

Port8080 lets you view and control the primary display of the Mac or Windows computer running it, from Chrome on another device. The host opens a small local page with a temporary Cloudflare Quick Tunnel link and a **Stop access** button.

## Start

Open `Start Port8080.command` on Apple silicon Mac, or double-click `Start-Port8080.bat` on Windows x64. The release packages include the host program, FFmpeg capture helper, and Cloudflare Tunnel helper; Go and separate installs are not required.

On macOS, allow Port8080 screen recording and accessibility access in System Settings when prompted, then restart Port8080. The Mac package is unsigned, so macOS may ask you to confirm opening it once. On Windows, SmartScreen may show an unsigned-app warning.

The host page is at `http://127.0.0.1:8766/host`. Only one viewer can control the host at a time. The temporary link is the only access secret: anyone with it can control the computer while Port8080 is running. Share it privately and use **Stop access** when finished.

## Early release limits

The stream uses direct WebRTC with public STUN. Networks that require a TURN relay may not connect. Windows first tries native Desktop Duplication capture, D3D11 GPU scaling, and an available hardware H.264 encoder; it falls back to the compatibility capture path if the native driver path fails. The default stream is capped at 1280×720, 30 frames per second, and a 6 Mbps video target. Audio and clipboard sync are not included in this package.

## Building from source

Install Go 1.24+, FFmpeg, and `cloudflared`, then run `go run .` from this folder. The packaged release includes those runtime helpers.
