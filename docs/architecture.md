# Architecture

## Decision: fork Yarr, keep its backend, replace its interface

Before writing code, Yarr's source was inspected (repository `nkanaev/yarr`, release **v2.9**, plus the 31 commits on `master` after it).

| Area | Finding | Decision |
|---|---|---|
| Backend | Go, standard library HTTP server, no framework | Keep |
| Storage | SQLite (WAL) with 15 versioned migrations; optional PostgreSQL behind a `Storage` interface | Keep, **no schema changes** |
| Feeds | RSS 0.9x/2.0, RDF, Atom, JSON Feed parser with fixtures; ETag/Last-Modified; favicon discovery | Keep |
| Pagination | Keyset (`after=<id>` on `(date, id)`), so pages can't overlap | Keep, add `limit` |
| HTML safety | Allow-list sanitizer derived from Miniflux/DOMPurify lists | Keep, add tracking-pixel removal |
| Scheduler | One ticker; idle between refreshes | Keep |
| OPML, Fever API, auth | Present | Keep |
| Frontend | Vue 3 + TypeScript + esbuild, three-pane desktop layout, Bootstrap CSS | **Replace** the UI, keep the toolchain |
| `master` after v2.9 | Mid-refactor: "read here" page fetching temporarily commented out | Base on the **v2.9 tag** |

A rewrite would have re-implemented a mature parser, migrations and a sanitizer for no user-visible benefit. The fork keeps Yarr's full git history and an `upstream` remote so future Yarr fixes can be merged. The Go module path stays `github.com/nkanaev/yarr` for the same reason (renaming it would touch every import and make upstream merges conflict-heavy); the binary and branding are Yarrow.

## What changed

### Backend (Go)

All new server code lives in separate files where possible (`src/server/yarrow.go`, `src/server/middleware/security.go`, `src/worker/netguard.go`, `src/content/preview`, `src/migrate`) to keep merges with upstream simple.

- **Card timeline API.** `GET /api/items?view=cards&limit=N` returns, per article, `snippet` (plain text from the article itself, ~320 chars, cut at a word) and `image` (feed media image, else the first reasonably sized `<img>`), without the full HTML body. Relative links are resolved. Media-only items fall back to the enclosure description.
- **Mark above as read.** `PUT /api/items?above_id=<id>[&folder_id|feed_id][&oldest_first=true]` marks everything shown above an item in the current timeline. Starred items are never touched.
- **Settings.** New `nav_mode` key (`folders` | `expanded` | `all`); all enumerated settings are validated before saving. Defaults for new installs: theme follows the device, refresh hourly.
- **Health.** `GET /healthz` (no auth) pings the database. `yarrow healthcheck` probes it from inside the container, so the image needs no curl.
- **PWA.** `/manifest.json` with PNG and maskable icons, `/sw.js` served from the app root so its scope covers the app (and the base path).
- **Static assets** get content-hash ETags.
- **Subcommands:** `migrate` (verified Yarr import) and `backup` (`VACUUM INTO` snapshot).
- **Feed add** returns the discovery error and fetches the favicon immediately.
- **OPML import** now returns 400 for a missing file and caps uploads at 10 MiB.
- **SQLite concurrency fix.** Yarr opened file databases in shared-cache mode with deferred transactions; concurrent writes during a refresh (items + favicons) could fail with "database is locked" and silently drop a feed's new articles until the next refresh. File databases now use private caches and `BEGIN IMMEDIATE` so writers wait on the busy timeout instead. A regression test hammers a file database from 16 goroutines.

### Security

| Threat | Control |
|---|---|
| Stored XSS from feed content | Server-side allow-list sanitizer on every article body and media description (unchanged from Yarr), plus a strict CSP: `script-src 'self'`, `style-src 'self'`, `object-src 'none'`, `frame-ancestors 'none'`. The app no longer uses inline scripts (config is a JSON data block) or Vue's runtime template compiler, so no `unsafe-inline`/`unsafe-eval` is needed. Browser tests open an article full of hostile markup and confirm nothing runs. |
| CSRF | Go's `http.CrossOriginProtection` rejects cross-site state-changing requests using `Sec-Fetch-Site`/`Origin`. Important because a password-less instance on a tailnet has no cookie to protect it. Fever API (api-key auth) is exempt. |
| SSRF via feed URLs, redirects, DNS rebinding, "Load full article" | `net.Dialer.Control` checks the IP actually being connected to, for every hop: loopback, RFC 1918, link-local/metadata, CGNAT (incl. Tailscale), ULA, NAT64/6to4/Teredo, multicast and reserved ranges are refused. Only http/https, no embedded credentials, max 10 redirects, 16 MiB response cap. Opt-in allow-list via `YARR_ALLOWED_NETWORKS`. |
| Session cookie | `HttpOnly`, `SameSite=Lax`, `Secure` whenever the request arrived over HTTPS (directly or via `X-Forwarded-Proto`). Yarr always set `Secure`, which broke login over plain-HTTP LAN addresses. |
| Tracking / privacy | No analytics or third-party requests from the app; `Referrer-Policy: no-referrer`; feed images load with `referrerpolicy=no-referrer`; 1×1 tracking pixels and known tracker hosts are stripped. |
| SQL injection | Parameterised queries throughout (unchanged). |
| Container | Non-root uid 1000, read-only root filesystem, all capabilities dropped, `no-new-privileges`, memory cap, published on `127.0.0.1` only by default. |

Known limitations: feed images are loaded directly from publishers' servers (no image proxy), so publishers see your IP when images load. Yarr's auth cookie is a static HMAC of the credentials (no server-side sessions or expiry beyond one week); unchanged from Yarr.

### Frontend (Vue 3, TypeScript, esbuild)

```
src/frontend/
  css/app.css            design tokens (light, sepia, dark, auto) and components
  js/main.ts             boot: reads #app-config JSON, theme, login vs app, SW registration
  js/store.ts            reactive state + actions (timeline, reader, read state, menus)
  js/manage.ts           feed/folder management (Yarr's menu actions)
  js/keys.ts             keyboard shortcuts
  js/theme.ts            theme + browser chrome colour
  js/pages/App.vue       shell: sidebar (desktop), tab bar (phone), sheets, menus
  js/components/
    TopBar.vue           large title, folder chips, All/Unread, search, view menu
    Timeline.vue         card list, infinite scroll, pull-to-refresh, states
    ArticleCard.vue      a card
    Reader.vue           article overlay
    NavList.vue          folder / expanded / all navigation
    Settings.vue         settings and feed management
    MenuHost.vue         one menu component: popover on desktop, bottom sheet on phones
    AddFeed.vue, Sheet.vue, Favicon.vue, Icon.vue, RelTime.vue
  sw/sw.ts               service worker
```

Design notes:

- **Desktop reading pane.** At ≥1200px the reader becomes a fixed pane on the right (`--reader-w: clamp(520px, 47vw, 880px)`) and the timeline keeps the window's scroll, so infinite scroll, scroll position and keyboard navigation work exactly as on phones. No history entry is pushed per article in this mode; `j/k` open articles directly, space scrolls the pane, Esc clears it. The choice between "beside the list" and "full page" is stored per browser (`localStorage`). Hover-only quick actions use `@media (hover: hover) and (pointer: fine)`, so touch devices never get them; right-click opens the same menu as ⋯.
- **Scroll preservation.** The reader is a fixed overlay; the timeline stays mounted underneath, so closing an article returns to the exact spot. Opening pushes a history entry, so the back button/gesture closes it.
- **Read state** changes only on open or explicit action. Items that become read stay in an Unread list until it is reloaded, so nothing jumps.
- **Navigation modes** map onto the same views: folder-only shows folder chips (phone) and folders (sidebar); expanded adds feeds under folders using Yarr's persisted `is_expanded`; all-feeds hides folder navigation.
- **Yarr compatibility.** The selected view and filter are stored in Yarr's own `feed`/`filter` settings, theme names are Yarr's (`night` is dark), and status semantics are Yarr's (unread/read/starred).
- **Offline.** App shell cached (stale-while-revalidate); read-only API calls network-first with a cached fallback marked by an `x-yarrow-offline` header that the UI turns into a banner. Writes are never cached. Cache names include a build hash, so updates roll out on the next load.
- **Not kept from Yarr's UI:** resizable three columns and the translation system (Yarrow's UI is English only).

### Repository layout

```
cmd/yarr/            entry point (binary is named yarrow), subcommands
src/server/          HTTP API (+ yarrow.go additions), middleware
src/storage/         SQLite and PostgreSQL storage, model, tests
src/worker/          fetching, scheduler, network guard
src/parser/          feed + OPML parsers (upstream)
src/content/         sanitizer, readability, preview (new)
src/migrate/         Yarr import and backup
src/frontend/        web app (see above); built into src/assets/static
tests/ui/            Playwright browser tests
scripts/             demo feed generator, offline Docker smoke test
docs/                guides and screenshots
doc/                 upstream Yarr documentation site (unchanged)
Dockerfile, compose.yaml, .env.example, Makefile, .github/workflows/ci.yml
```
