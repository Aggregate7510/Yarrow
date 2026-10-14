# Testing

## How to run

```sh
make test          # vue-tsc typecheck, go vet, Go unit/integration tests
make test-ui       # browser tests (needs: pip install playwright && python -m playwright install chromium)
make docker-smoke  # offline container smoke test (static binary in a scratch image + real compose.yaml)
docker compose config -q
```

CI (`.github/workflows/ci.yml`) runs all of the above on every push, then builds the real Docker image separately for `linux/amd64` and `linux/arm64` (QEMU), starts it with `compose.yaml`, waits for the health check, checks data survives container re-creation and that only `127.0.0.1:7070` is published.

## What the tests cover

**Go** (`go test ./...`, 13 packages): Yarr's existing parser, sanitizer, storage, Fever and server tests, plus:

- network guard: blocked ranges (loopback, RFC 1918, metadata, CGNAT, IPv6 forms), allow-list, loopback and `localhost` blocked at dial time, redirect to an internal address blocked, bad schemes and credentials refused, response size cap
- preview: text extraction (scripts/styles skipped), truncation incl. multibyte, title de-duplication, image choice (media first, srcset, emoji/tracker/1×1/data: skipped, unsafe schemes refused)
- sanitizer: tracking pixels removed
- server: card view fields and no full content, pagination over 45 items with equal timestamps (no duplicates), limit clamping, invalid status, mark-above-as-read, settings validation, CSRF (cross-site rejected, same-origin and non-browser allowed), security headers, no inline script, manifest/service worker under a base path, static ETag/304, cookie flags over HTTP and behind HTTPS proxy, auth on API but not on `/healthz`
- storage: mark-above in both sort orders and within a folder, starred items untouched, folder timeline keyset pagination, `nav_mode` round trip, settings validation
- migration: everything preserved, source byte-for-byte unchanged, refusal to overwrite, overwrite moves aside, bad inputs, live backup while a writer is open

**Browser** (`tests/ui/run_ui_tests.py`, 21 tests, Chromium via Playwright, iPhone 390×844 / iPad 820×1180 / desktop 1440×900):

| Area | Checks |
|---|---|
| Timeline | cards with titles, snippets, images and image-less cards; no horizontal overflow; no console or CSP errors |
| Long headlines | wrap on a phone without overflow |
| Folder-only navigation | chips list folders, folder sheet shows no feeds, folder timeline only contains that folder's feeds |
| Large feed | 1,500-item feed paged by infinite scroll, no duplicate cards |
| Scroll position | preserved after closing the reader via Back and via browser back |
| Read state | scrolling does not mark read; opening does |
| Menus | article menu entries; save for later → appears in Saved; read/unread toggle; mark above as read |
| Filters / search | Unread filter, search within a view |
| Empty folder | empty state with guidance |
| Security | hostile article: no script execution, no event handlers, `javascript:` URLs, foreign iframes or inline styles survive; tracking pixel removed |
| Themes | light, dark, sepia, and Auto following the OS setting |
| Tablet, desktop | layout widths, sidebar, expanded and all-feeds modes, keyboard `j/k/o/Esc/?` |
| Settings | create/rename/delete folder, move and rename feed, navigation mode and refresh interval persist |
| Add / remove feed | add into a folder, private address refused with a clear message, unsubscribe |
| Slow network and errors | throttled connection shows skeletons; API failure shows an error with a working retry |
| OPML export, PWA | export contains folders; manifest icons served; service worker activates |

Screenshots in `docs/screenshots/` are produced by this suite.

**Manually verified in this environment:**

- Real Yarr **v2.4** database (schema v8, built from the upstream tag) migrated inside the container to schema v15; counts, starred items and settings verified; source checksum unchanged.
- Unmodified Yarr **v2.9** opens a Yarrow database (rollback path).
- `compose.yaml` with the smoke image: healthy, non-root, read-only rootfs, capabilities dropped, published only on `127.0.0.1:7070`, data persisted across `docker restart` and `docker compose down`/`up`.
- Idle memory about 25 MB; a 25-card timeline page takes about 5 ms to serve on x86 and is ~1.9 KB gzipped.

## Not verified here (be aware)

These could not be done in the development environment, which had no access to container registries, external websites or Apple devices:

- **The real `Dockerfile` has not been built here** (base images `node`, `golang` and `alpine` could not be pulled). Its stages were exercised piecewise: the same build commands and tags were run on the host, and the runtime contract was tested with an equivalent scratch image. CI builds the real image for both architectures.
- **ARM64 has not been run on Raspberry Pi hardware.** CI runs the arm64 image under QEMU emulation, which validates the binary and container, not Pi-specific performance or storage behaviour.
- **iOS Safari / WebKit** was not tested; Playwright's WebKit build could not be downloaded. Phone tests use Chromium with iPhone viewport, touch and user agent. Things worth checking on a real iPhone: home-screen install, safe-area insets, pull-to-refresh feel, the collapsing header.
- **Tailscale Serve** was not run; the header behaviour Yarrow relies on was confirmed from Tailscale's source code.
- **Real-world feeds** from the internet were not fetched (outbound access was blocked); all feed tests use generated local RSS/Atom feeds, plus Yarr's own parser fixtures.
- **PostgreSQL** storage compiles and gained the same small changes as SQLite, but its tests need a Postgres container and were not run.
