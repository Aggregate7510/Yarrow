# Yarrow build tasks. Requires Go (see go.mod), a C compiler (SQLite uses cgo)
# and Node.js 20+ for the web assets.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GITHASH ?= $(shell git rev-parse --short=8 HEAD 2>/dev/null || echo unknown)

GO_TAGS    = sqlite_foreign_keys sqlite_json sqlite_fts5 sqlite_omit_load_extension
GO_LDFLAGS = -s -w -X 'main.Version=$(VERSION)' -X 'main.GitHash=$(GITHASH)'
GO_FLAGS   = -mod=vendor -trimpath -tags "$(GO_TAGS)" -ldflags="$(GO_LDFLAGS)"

IMAGE     ?= yarrow
PLATFORMS ?= linux/amd64,linux/arm64

export CGO_ENABLED=1

.PHONY: default deps assets build test test-go test-ui typecheck vet serve \
        docker docker-multiarch docker-smoke demo clean

default: test build

node_modules: package-lock.json
	npm ci --no-audit --no-fund
	@touch node_modules

deps: node_modules

assets: node_modules
	npm run build -- --minify

build: assets
	go build $(GO_FLAGS) -o out/yarrow ./cmd/yarr

typecheck: node_modules
	npx vue-tsc --noEmit

vet:
	go vet -mod=vendor -tags "$(GO_TAGS)" ./...

test-go: assets
	go test -mod=vendor -short -tags "$(GO_TAGS)" ./...

test-ui: build
	python3 tests/ui/run_ui_tests.py --binary out/yarrow

test: typecheck vet test-go

# local development server with live-reloaded assets (run `npm run watch` too)
YARR_DB ?= local.db
serve: assets
	go run -mod=vendor -tags "$(GO_TAGS) debug" ./cmd/yarr -db "$(YARR_DB)"

# demo feeds served from localhost; import /tmp/yarrow-demo/demo.opml afterwards
demo:
	python3 scripts/demo/generate.py /tmp/yarrow-demo http://127.0.0.1:8099
	@echo "serve with: python3 -m http.server 8099 --bind 127.0.0.1 --directory /tmp/yarrow-demo"
	@echo "then run:   YARR_ALLOWED_NETWORKS=127.0.0.1 ./out/yarrow -db demo.db"

docker:
	docker build --build-arg VERSION=$(VERSION) --build-arg GITHASH=$(GITHASH) -t $(IMAGE):$(VERSION) -t $(IMAGE):local .

# cross-build for the Raspberry Pi and amd64 servers (needs buildx + QEMU binfmt)
docker-multiarch:
	docker buildx build --platform $(PLATFORMS) --build-arg VERSION=$(VERSION) \
	  --build-arg GITHASH=$(GITHASH) -t $(IMAGE):$(VERSION) .

docker-smoke:
	scripts/docker-smoke/run.sh

clean:
	rm -rf out src/assets/static/bundle.* src/assets/static/sw.js
