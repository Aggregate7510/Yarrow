/// <reference lib="webworker" />
// Yarrow service worker.
//
// - App shell (page, bundle, icons): served from cache, refreshed in the
//   background, so the app opens instantly from the home screen.
// - Read-only API calls (timeline pages, article bodies, feed list): network
//   first, falling back to the last cached copy when offline. Fallback
//   responses carry `x-yarrow-offline: 1` so the UI can say so.
// - Anything that changes data, logs in or out, or is cross-origin (article
//   images) always goes straight to the network and is never cached.

declare const self: ServiceWorkerGlobalScope;
declare const __CACHE_VERSION__: string;

const SHELL = "yarrow-shell-" + __CACHE_VERSION__;
const DATA = "yarrow-data-v1";
const MAX_DATA_ENTRIES = 400;

const scope = new URL(self.registration.scope);
const base = scope.pathname; // e.g. "/" or "/rss/"
const SHELL_URLS = [base, base + "static/bundle.js", base + "static/bundle.css", base + "static/icons/icon-192.png"];

self.addEventListener("install", event => {
  event.waitUntil(
    caches
      .open(SHELL)
      .then(c => c.addAll(SHELL_URLS))
      .catch(() => {}) // e.g. not signed in yet; caching resumes on use
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", event => {
  event.waitUntil(
    (async () => {
      for (const key of await caches.keys()) {
        if (key.startsWith("yarrow-shell-") && key !== SHELL) await caches.delete(key);
      }
      await self.clients.claim();
    })(),
  );
});

function isCacheableApi(url: URL): boolean {
  const p = url.pathname.slice(base.length - 1);
  return p === "/api/items" || /^\/api\/items\/\d+$/.test(p) || p === "/api/feeds" || p === "/api/folders" || p === "/api/status" || p === "/api/settings";
}

async function trimData() {
  const cache = await caches.open(DATA);
  const keys = await cache.keys();
  for (let i = 0; i < keys.length - MAX_DATA_ENTRIES; i++) await cache.delete(keys[i]);
}

function markOffline(res: Response): Response {
  const headers = new Headers(res.headers);
  headers.set("x-yarrow-offline", "1");
  return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
}

async function networkFirst(req: Request): Promise<Response> {
  try {
    const res = await fetch(req);
    if (res.ok) {
      const cache = await caches.open(DATA);
      await cache.put(req, res.clone());
      trimData();
    }
    return res;
  } catch (err) {
    const cached = await caches.match(req, { cacheName: DATA });
    if (cached) return markOffline(cached);
    throw err;
  }
}

async function staleWhileRevalidate(req: Request): Promise<Response> {
  const cache = await caches.open(SHELL);
  const cached = await cache.match(req, { ignoreSearch: true });
  const fresh = fetch(req)
    .then(res => {
      if (res.ok && res.type === "basic") cache.put(req, res.clone());
      return res;
    })
    .catch(() => undefined);
  return cached ?? (await fresh) ?? Response.error();
}

self.addEventListener("fetch", event => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== scope.origin || !url.pathname.startsWith(base)) return;

  if (req.mode === "navigate") {
    // always try for a fresh page (it embeds settings and auth state)
    event.respondWith(
      fetch(req)
        .then(res => {
          if (res.ok) caches.open(SHELL).then(c => c.put(base, res.clone()));
          return res;
        })
        .catch(async () => (await caches.match(base, { cacheName: SHELL })) ?? Response.error()),
    );
    return;
  }
  if (url.pathname.startsWith(base + "static/")) {
    event.respondWith(staleWhileRevalidate(req));
    return;
  }
  if (isCacheableApi(url)) {
    event.respondWith(networkFirst(req));
  }
});
