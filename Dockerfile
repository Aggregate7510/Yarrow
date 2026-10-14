# syntax=docker/dockerfile:1
#
# Yarrow — multi-stage, multi-architecture image (linux/amd64, linux/arm64).
#
#   docker build -t yarrow .
#   docker buildx build --platform linux/amd64,linux/arm64 -t yarrow .
#
# Stage 1 builds the web assets once on the build machine's own platform
# (they are architecture independent). Stage 2 compiles a fully static Go
# binary (SQLite needs cgo) for the target platform. The runtime image is a
# small Alpine with a non-root user.

ARG GO_VERSION=1.26
ARG NODE_VERSION=22
ARG ALPINE_VERSION=3.22

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS assets
WORKDIR /src
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY build.js tsconfig.json ./
COPY src/frontend ./src/frontend
COPY src/assets ./src/assets
RUN npm run build -- --minify

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY cmd ./cmd
COPY src ./src
COPY --from=assets /src/src/assets/static ./src/assets/static
ARG VERSION=dev
ARG GITHASH=unknown
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -mod=vendor -trimpath \
      -tags "sqlite_foreign_keys sqlite_json sqlite_fts5 sqlite_omit_load_extension netgo osusergo" \
      -ldflags "-s -w -linkmode external -extldflags '-static' -X main.Version=${VERSION} -X main.GitHash=${GITHASH}" \
      -o /out/yarrow ./cmd/yarr \
 && /out/yarrow -version

FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 1000 yarrow \
 && adduser -S -D -H -u 1000 -G yarrow yarrow \
 && mkdir -p /data \
 && chown yarrow:yarrow /data
COPY --from=build /out/yarrow /usr/local/bin/yarrow
USER 1000:1000
ENV YARR_ADDR=0.0.0.0:7070 \
    YARR_DB=/data/storage.db
VOLUME /data
EXPOSE 7070
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD ["/usr/local/bin/yarrow", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/yarrow"]
