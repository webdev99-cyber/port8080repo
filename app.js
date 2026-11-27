(() => {
  const macButton = document.querySelector('#download-mac');
  const winButton = document.querySelector('#download-windows');
  const releaseNote = document.querySelector('#release-note');

  const setDownload = (platform, assetName, release) => {
    const button = document.querySelector(`#download-${platform}`);
    const status = document.querySelector(`#status-${platform}`);
    const asset = release.assets.find((item) => item.name.toLowerCase() === assetName.toLowerCase());
    if (!asset) {
      status.textContent = 'Not included in the latest release';
      return;
    }
    button.href = asset.browser_download_url;
    button.setAttribute('aria-disabled', 'false');
    button.setAttribute('download', '');
    status.textContent = `${release.tag_name || 'Latest release'} · ${(asset.size / 1024 / 1024).toFixed(1)} MB`;
  };

  fetch('./site-config.json', { cache: 'no-store' })
    .then((response) => {
      if (!response.ok) throw new Error('Site configuration is unavailable');
      return response.json();
    })
    .then(async (config) => {
      const repo = typeof config.repository === 'string' ? config.repository.trim() : '';
      if (!/^[\w.-]+\/[\w.-]+$/.test(repo)) return;

      releaseNote.textContent = 'Checking the latest Port8080 release…';
      const response = await fetch(`https://api.github.com/repos/${repo}/releases/latest`, {
        headers: { Accept: 'application/vnd.github+json' },
      });
      if (!response.ok) throw new Error('No public release found yet');
      const release = await response.json();
      setDownload('mac', config.assets.mac, release);
      setDownload('windows', config.assets.windows, release);
      releaseNote.textContent = `Downloads from ${release.tag_name || 'the latest release'} · View the release on GitHub`;
      const link = document.createElement('a');
      link.href = release.html_url;
      link.textContent = 'View the release on GitHub';
      link.target = '_blank';
      link.rel = 'noopener noreferrer';
      releaseNote.replaceChildren(document.createTextNode(`${release.tag_name || 'Latest release'} · `), link);
    })
    .catch(() => {
      macButton.setAttribute('aria-disabled', 'true');
      winButton.setAttribute('aria-disabled', 'true');
      releaseNote.textContent = 'Downloads will appear here after Port8080 installers are published.';
    });
})();
