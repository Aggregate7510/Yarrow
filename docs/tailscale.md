# Private access with Tailscale

The recommended way to use Yarrow away from home: keep it bound to `127.0.0.1` (the default in `compose.yaml`) and let Tailscale Serve publish it over HTTPS to devices on your tailnet only. Nothing is opened on your router or to the internet.

## Set up

On the machine running Yarrow (e.g. the Pi), with Tailscale already installed and logged in:

1. In the Tailscale admin console, enable **MagicDNS** and **HTTPS certificates** (DNS page).
2. Publish Yarrow on your tailnet:

   ```sh
   sudo tailscale serve --bg http://127.0.0.1:7070
   tailscale serve status
   ```

   This serves `https://<machine-name>.<your-tailnet>.ts.net/` → `http://127.0.0.1:7070` and persists across reboots.

3. On your iPhone (with the Tailscale app connected) open `https://<machine-name>.<your-tailnet>.ts.net/` and **Add to Home Screen**.

To stop publishing: `sudo tailscale serve --https=443 off`.

> Use **Serve**, not **Funnel**. Funnel would put Yarrow on the public internet.

## Serving under a sub-path

If other apps already use the root of that hostname, serve Yarrow under a path and tell Yarrow its base path:

```sh
# .env
YARR_BASE=/rss
```

```sh
docker compose up -d
sudo tailscale serve --bg --set-path /rss http://127.0.0.1:7070/rss
```

Open `https://<machine>.<tailnet>.ts.net/rss/`. The PWA scope follows the base path.

## How Yarrow behaves behind the proxy

- All frontend requests use relative URLs, so there is no hard-coded host or port and no mixed content.
- Tailscale Serve sets `X-Forwarded-Proto: https` on proxied requests (see `ipn/ipnlocal/serve.go` in the Tailscale source). Yarrow uses it only to mark its login cookie `Secure`; the cookie is always `HttpOnly` and `SameSite=Lax`.
- CSRF protection uses the browser's `Sec-Fetch-Site` / `Origin` headers, which are independent of how the proxy rewrites `Host`.
- `/healthz` is the only unauthenticated endpoint besides the login page and static assets.

## Do I still need a password?

Tailscale already restricts who can reach the address, so a password is optional. Set `YARR_AUTH=user:password` in `.env` if other people share your tailnet, or if you ever bind Yarrow to the LAN.

## Other reverse proxies

Any proxy that forwards to `127.0.0.1:7070` works. For Caddy:

```
rss.example.internal {
    reverse_proxy 127.0.0.1:7070
}
```

For nginx, include `proxy_set_header X-Forwarded-Proto $scheme;` so the cookie is marked `Secure`.
