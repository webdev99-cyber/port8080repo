# Build Port8080 for macOS (free)

The Mac download is currently an unsigned app, so this guide builds Port8080 on your own Mac instead. This does not require an Apple Developer Program membership.

## What you need

- A Mac with macOS and Xcode Command Line Tools
- Homebrew
- An internet connection

## Steps

1. Open Terminal.
2. If you do not already have the Xcode Command Line Tools, run:

   ```sh
   xcode-select --install
   ```

   Follow the installer prompts. If they are already installed, continue.

3. Install Go, FFmpeg, and Cloudflare Tunnel:

   ```sh
   brew install go ffmpeg cloudflared
   ```

4. Download the Port8080 source and start it:

   ```sh
   git clone https://github.com/webdev99-cyber/port8080repo.git
   cd port8080repo/app
   go run .
   ```

   Go compiles and runs the host from source. Keep Terminal open while Port8080 is running.

5. Allow screen recording and accessibility access if macOS asks, then restart Port8080 if needed. The host page opens at `http://127.0.0.1:8766/host`.

## Keep the session private

The active link allows its holder to control your computer. Share it only with someone you trust, never post it publicly, and press **Stop access** when finished. Closing Port8080 also stops the host.

## Notes

This is an early release. Some networks may not connect, and performance depends on the Mac and network. Port8080 currently targets Apple silicon for its prebuilt Mac package; building from source may work on other Macs, but that has not been verified.
