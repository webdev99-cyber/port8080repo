const REPOSITORY = "webdev99-cyber/port8080repo";
const BRANCH = "main";
const FILES = new Set(["index.html", "styles.css", "app.js", "site-config.json", "favicon.svg"]);
const TYPES = {
  "index.html": "text/html; charset=utf-8",
  "styles.css": "text/css; charset=utf-8",
  "app.js": "text/javascript; charset=utf-8",
  "site-config.json": "application/json; charset=utf-8",
  "favicon.svg": "image/svg+xml",
};

export default {
  async fetch(request) {
    const url = new URL(request.url);
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response("Method not allowed", { status: 405, headers: { Allow: "GET, HEAD" } });
    }

    let file = url.pathname === "/" ? "index.html" : url.pathname.slice(1);
    try { file = decodeURIComponent(file); } catch { return new Response("Not found", { status: 404 }); }
    if (!FILES.has(file)) return new Response("Not found", { status: 404 });

    const upstream = `https://raw.githubusercontent.com/${REPOSITORY}/${BRANCH}/${file}`;
    const response = await fetch(upstream, { cf: { cacheTtl: file === "site-config.json" ? 0 : 60 } });
    if (!response.ok) return new Response("Site file unavailable", { status: 502 });

    const headers = new Headers({
      "Content-Type": TYPES[file],
      "Cache-Control": file === "site-config.json" ? "no-store" : "public, max-age=60",
      "X-Content-Type-Options": "nosniff",
      "Referrer-Policy": "strict-origin-when-cross-origin",
      "X-Frame-Options": "DENY",
      "Content-Security-Policy": "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' https://api.github.com; font-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'",
    });
    return new Response(request.method === "HEAD" ? null : response.body, { status: 200, headers });
  },
};
