# Port8080 website

This is the static website source for [port8080.dpdns.org](https://port8080.dpdns.org/). Cloudflare Worker `port8080repo` serves these files from the `main` branch of this repository. Site edits become available after the Worker’s short cache expires; no GitHub Pages project or build step is needed.

## Downloads

The page reads the latest public GitHub Release and enables a download button only when its matching installer is attached:

- `Port8080-macOS-arm64.dmg`
- `Port8080-Windows-x64.zip`

Until those release assets exist, the buttons stay disabled. The site does not make up download links or include installers in the repository.

## Cloudflare Worker

`_worker.js` is the Worker entrypoint. It serves only the five public site files in this folder by fetching them from this repository’s `main` branch. The live Worker script can be updated through Cloudflare’s Worker API or `wrangler deploy` while keeping the existing `port8080repo` route.
