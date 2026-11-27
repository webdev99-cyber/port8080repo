# Port8080 website

Static GitHub Pages site files for Port8080.

## Setup

1. Put these files in the root of the Port8080 GitHub repository.
2. Edit `site-config.json` and replace `YOUR_GITHUB_USERNAME/port8080` with the exact `OWNER/REPOSITORY` slug.
3. In GitHub, open **Settings → Pages** and select **GitHub Actions** as the deployment source. The included workflow deploys the site on pushes to `main`.
4. Publish GitHub Releases with these assets for the download buttons to activate:
   - `Port8080-macOS-arm64.dmg`
   - `Port8080-Windows-x64.zip`

The page reads the latest public GitHub Release at runtime. It does not contain installers, and it keeps the download buttons disabled until matching release assets exist. Once Pages is enabled and the workflow succeeds, the expected site URL is `https://webdev99-cyber.github.io/port8080repo/`.

## Content notes

The page describes Port8080 as an early preview. It avoids promising guaranteed frame rate or network connectivity and explains that the active host link grants control of the computer.
