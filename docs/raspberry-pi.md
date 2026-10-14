# Raspberry Pi deployment

Yarrow runs as a single container on a Raspberry Pi with a **64-bit** OS (Raspberry Pi OS Bookworm 64-bit or Ubuntu Server arm64). A Pi 3B+ is enough; a Pi 4 or 5 is comfortable. Idle memory use is around 25 MB.

> **Verification status.** The image build and container behaviour were verified on amd64 with an equivalent static build. The `linux/arm64` image is built and health-checked in CI under QEMU emulation, but it has **not yet been run on physical Raspberry Pi hardware**. Please report anything that differs on a real Pi.

## 1. Check the OS is 64-bit

```sh
uname -m          # must print aarch64
getconf LONG_BIT  # must print 64
```

If it prints `armv7l`, reinstall with the 64-bit Raspberry Pi OS image (Raspberry Pi Imager → *Raspberry Pi OS (64-bit)*).

## 2. Install Docker Engine and the Compose plugin

```sh
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker "$USER"
# log out and back in, then:
docker version
docker compose version
```

## 3. Get Yarrow and start it

```sh
git clone <your-fork-url> ~/yarrow
cd ~/yarrow
cp .env.example .env     # optional; see below
docker compose up -d --build
```

The first build compiles Yarrow on the Pi and takes roughly 5–15 minutes depending on the model and SD card speed. Later builds reuse Docker's cache.

Check it is healthy:

```sh
docker compose ps                 # STATUS should say (healthy)
curl -s http://127.0.0.1:7070/healthz
```

## 4. Reach it from your phone

By default the port is published on `127.0.0.1` only, so nothing is exposed to your network. Choose one:

- **Tailscale (recommended):** follow [tailscale.md](tailscale.md). You get `https://<pi-name>.<tailnet>.ts.net` on all your devices with a real certificate and no open ports.
- **Home network:** in `.env` set `YARROW_BIND=0.0.0.0` and `YARR_AUTH=yourname:a-long-password`, then `docker compose up -d`. Open `http://<pi-ip>:7070`. Without HTTPS the app works, but the iPhone home-screen app cannot use offline mode.

## 5. Install on the iPhone home screen

Open the HTTPS address in Safari → Share → **Add to Home Screen**. Yarrow opens full-screen with its own icon.

## Updating

```sh
cd ~/yarrow
git pull
docker compose up -d --build
```

Your data lives in the `yarrow_yarrow-data` volume and survives rebuilds. Take a backup first if you like ([backup-restore.md](backup-restore.md)).

## Building on a faster machine instead

To avoid compiling on the Pi, build the arm64 image on a PC with Docker Buildx and copy it over:

```sh
docker buildx build --platform linux/arm64 -t yarrow:local --output type=docker,dest=yarrow-arm64.tar .
scp yarrow-arm64.tar pi@<pi>:~
# on the Pi
docker load -i ~/yarrow-arm64.tar
cd ~/yarrow && docker compose up -d --no-build
```

Cross-building needs QEMU binfmt on the PC (`docker run --privileged --rm tonistiigi/binfmt --install arm64`).

## Resource notes

- Feeds refresh on a timer (default every hour; change it in Settings). Between refreshes Yarrow is idle; nothing polls.
- Articles older than 90 days are pruned per feed (keeping at least the newest 50, never saved ones), as in Yarr.
- `compose.yaml` caps the container at 256 MB of memory and keeps logs to 15 MB.
- SQLite runs in WAL mode. An SSD over USB lasts longer than an SD card for any always-on service, but an SD card works.
