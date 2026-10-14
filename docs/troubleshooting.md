# Troubleshooting

## The container keeps restarting or is "unhealthy"

```sh
docker compose logs --tail=50 yarrow
docker inspect --format '{{json .State.Health}}' yarrow
```

- **`Failed to initialise database: disk I/O error: permission denied`**: the data directory or `/tmp` isn't writable by uid 1000. With the default named volume this doesn't happen. If you changed the volume to a bind mount (`./data:/data`), fix ownership: `sudo chown -R 1000:1000 ./data`. If you edited the `tmpfs` line, keep `mode=1777`.
- **`exec format error`**: the image was built for a different CPU. Rebuild on the Pi, or cross-build with `--platform linux/arm64`. Check `uname -m` prints `aarch64`; 32-bit Raspberry Pi OS is not supported.
- **Out of memory during the first run after importing a very large Yarr database**: the one-time schema upgrade can need more than the 256 MB limit. Temporarily remove the `deploy.resources` block in `compose.yaml`, start once, then restore it.

## I can't open it from my phone

- By default Yarrow listens on `127.0.0.1` on the host. That is intentional. Use [Tailscale Serve](tailscale.md), or set `YARROW_BIND=0.0.0.0` in `.env` and `docker compose up -d`.
- `curl -s http://127.0.0.1:7070/healthz` on the host should print `{"status":"ok"}`.
- With Tailscale, check `tailscale serve status` and that the phone's Tailscale app is connected.

## Login loops back to the login page

- Make sure the clock on the server is correct.
- If you serve Yarrow under a sub-path, set `YARR_BASE` to the same path the proxy uses.
- Clear the site's cookies, then sign in again.

## "Add feed" says the address is on a private network

The SSRF guard blocks non-public addresses. If the feed really is on your network, allow it with `YARR_ALLOWED_NETWORKS` ([configuration.md](configuration.md#fetching-from-your-own-network)).

## "No feed was found at that address"

Paste the feed URL itself (often ending in `/feed`, `/rss`, `.xml` or `atom.xml`) instead of the home page. Some sites block unknown user agents or require JavaScript; those can't be fetched by any server-side reader.

## A feed shows an error in Settings

The message under the feed in **Settings → Feeds and folders** is the last refresh error (HTTP status, TLS or parse error). Use the feed's ⋯ → **Change link** if it moved. Errors clear automatically after the next successful refresh.

## Articles have no images

Images come from the feed (`media:content`, enclosures, or the first sizeable `<img>` in the article). Feeds that publish only text show text-only cards. Images load directly from the publisher's site; if a site blocks hot-linking, the card hides the broken image.

## The snippet is short

If the feed publishes only a one-line summary, that summary is what Yarrow shows. Open the article and tap **Load full article** to extract the page's text.

## Offline mode / "Add to Home Screen" doesn't cache anything

Service workers only run on HTTPS (or `localhost`). Use the Tailscale HTTPS address. Offline mode keeps the app shell and the timeline pages and articles you've recently opened; it doesn't download everything.

## Migration: "open source read-only: unable to open database file"

- Stop Yarr first so it checkpoints its write-ahead log.
- If a `yarr.db-wal` file is still next to the database, SQLite needs to create a `-shm` file even for read-only access. Mount the directory without `:ro` (the tool still opens the database read-only and never modifies it), or open and close the database once with Yarr.

## Feeds don't refresh

Check **Settings → Reading → Refresh feeds** isn't set to *Manually*, and look for `auto-refresh` lines in `docker compose logs`. Pull down on the timeline (phone) or press `r` (desktop) to refresh now.

## Resetting

To start over completely: `docker compose down -v` deletes the container **and the data volume**. Back up first.
