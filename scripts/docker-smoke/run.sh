#!/usr/bin/env bash
# Offline container smoke test.
#
# Builds a static binary on the host, wraps it in a FROM-scratch image with
# the same runtime contract as ./Dockerfile (uid 1000, /data volume, env,
# HEALTHCHECK), tags it yarrow:local and drives the real compose.yaml with it:
# health check, published ports, data persistence across restart and
# re-creation. Use it where base images cannot be pulled; otherwise prefer
# `docker compose up -d --build`, which builds the real image.
set -euo pipefail
cd "$(dirname "$0")/../.."
work=$(mktemp -d)
trap 'docker compose down -v >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

TAGS="sqlite_foreign_keys sqlite_json sqlite_fts5 sqlite_omit_load_extension netgo osusergo"
CGO_ENABLED=1 go build -mod=vendor -trimpath -tags "$TAGS" \
  -ldflags "-s -w -linkmode external -extldflags -static -X main.Version=smoke" \
  -o "$work/yarrow" ./cmd/yarr
cp scripts/docker-smoke/Dockerfile "$work/"
cp /etc/ssl/certs/ca-certificates.crt "$work/"
mkdir -p "$work/data" "$work/tmp"
docker build -q -t yarrow:local "$work" >/dev/null

docker compose up -d --no-build
for i in $(seq 1 30); do
  [ "$(docker inspect --format '{{.State.Health.Status}}' yarrow)" = healthy ] && break
  sleep 1
done
echo "health:  $(docker inspect --format '{{.State.Health.Status}}' yarrow)"
echo "ports:   $(docker port yarrow | tr '\n' ' ')"
curl -fsS -X POST localhost:7070/api/folders -H 'content-type: application/json' -d '{"title":"persist-check"}' >/dev/null
docker restart yarrow >/dev/null; sleep 3
docker compose down >/dev/null; docker compose up -d --no-build >/dev/null; sleep 3
if curl -fsS localhost:7070/api/folders | grep -q persist-check; then
  echo "persist: OK (survived restart and container re-creation)"
else
  echo "persist: FAILED"; exit 1
fi
