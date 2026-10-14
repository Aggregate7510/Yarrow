# Configuration

Yarrow is configured with environment variables (or the equivalent flags). It keeps Yarr's variable names so existing setups carry over. Everything else is changed in the app under **Settings**.

## Server

| Variable | Flag | Default | Purpose |
|---|---|---|---|
| `YARR_ADDR` | `-addr` | `127.0.0.1:7070` (`0.0.0.0:7070` in the image) | Listen address, or `unix:/path/to.sock` |
| `YARR_DB` | `-db` | `~/.config/yarr/storage.db` (`/data/storage.db` in the image) | SQLite file (a `postgres://` URL also works, as in Yarr) |
| `YARR_BASE` | `-base` | *(empty)* | Serve under a sub-path such as `/rss` |
| `YARR_AUTH` | `-auth` | *(empty)* | `username:password` to require a login |
| `YARR_AUTHFILE` | `-auth-file` | *(empty)* | File containing `username:password` (takes precedence; good for Docker secrets) |
| `YARR_CERTFILE`, `YARR_KEYFILE` | `-cert-file`, `-key-file` | *(empty)* | Serve HTTPS directly (usually a proxy handles this) |
| `YARR_LOGFILE` | `-log-file` | stdout | Log file path |
| `YARR_ALLOWED_NETWORKS` | `-allowed-networks` | *(empty)* | Comma-separated CIDRs or IPs on private networks that feeds may be fetched from |

## Compose-only (`.env`)

| Variable | Default | Purpose |
|---|---|---|
| `YARROW_BIND` | `127.0.0.1` | Host address the port is published on. `0.0.0.0` exposes it on your LAN |
| `YARROW_PORT` | `7070` | Host port |
| `TZ` | `UTC` | Timezone for log timestamps |
| `YARROW_VERSION` | `local` | Tag of the locally built image |

## In-app settings

Stored in the database (`settings` table), shared by all devices:

- **Theme:** Light, Sepia, Dark, Auto (follows the device)
- **Article font and text size**
- **Navigation:** Folders (default), Expanded, All feeds
- **Show first:** newest or oldest
- **Refresh feeds:** manually, or every 10 min – once a day (default: every hour)

Per-device only: whether the desktop sidebar is collapsed.

## Fetching from your own network

To stop feeds or "Load full article" from being used to probe your network (SSRF), Yarrow refuses to connect to loopback, private (RFC 1918), link-local (including cloud metadata `169.254.169.254`), CGNAT/Tailscale `100.64.0.0/10`, unique-local IPv6 and other non-public ranges. The check runs on the address actually connected to, so DNS tricks and redirects can't bypass it.

If you subscribe to something on your LAN, such as an RSS-Bridge container, allow just that address or range:

```sh
YARR_ALLOWED_NETWORKS=192.168.1.20
YARR_ALLOWED_NETWORKS=192.168.1.0/24,10.0.0.5
```

If the feed runs in another container on the same Compose network, allow the Docker network's range (`docker network inspect yarrow_default`), typically something like `172.18.0.0/16`.

If you route outbound traffic through an HTTP proxy (`HTTPS_PROXY`), Yarrow connects to the proxy itself and the proxy performs the fetch; make sure the proxy enforces its own egress rules.

## Sizes and limits

- Remote responses (feeds, pages, icons) are capped at 16 MiB.
- OPML uploads are capped at 10 MiB.
- The timeline API returns at most 100 items per request (the app asks for 25).
