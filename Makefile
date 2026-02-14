VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := harbor
GOFLAGS := -trimpath -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build clean test lint install gateway sdk-build connector-build connector-bundle all dev-install

# ── Build ────────────────────────────────────────────────────────

build:
	go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/harbor

gateway:
	go build $(GOFLAGS) -o bin/harbor-gateway ./gateway

INSTALL_DIR ?= $(shell npm config get prefix 2>/dev/null || echo /usr/local)/bin

install: build
	cp bin/$(BINARY) $(INSTALL_DIR)/$(BINARY)

# ── SDK & Connectors ─────────────────────────────────────────────

sdk-build:
	cd sdk/typescript && npm install && npm run build

connector-build: sdk-build
	cd connectors/coingecko && npm install && npm run build

connector-bundle:
	esbuild connectors/coingecko/src/index.ts \
		--bundle --platform=node --target=node18 --format=cjs \
		--outfile=connectors/coingecko/dist/coingecko.js \
		--banner:js='#!/usr/bin/env node' \
		--alias:harbor-sdk=./sdk/typescript/src/index.ts

# ── All ──────────────────────────────────────────────────────────

all: build connector-bundle

# ── Dev workflow ─────────────────────────────────────────────────

dev-install: build connector-bundle
	./bin/harbor install coingecko --from connectors/coingecko/dist/coingecko.js
	@echo ""
	@echo "Done! Try:"
	@echo "  ./bin/harbor list"
	@echo "  ./bin/harbor get coingecko.prices --param ids=bitcoin --param vs_currencies=usd"
	@echo "  ./bin/harbor get coingecko.trending"
	@echo "  ./bin/harbor tools export"

# ── Test ─────────────────────────────────────────────────────────

test:
	go test ./... -v -race

lint:
	golangci-lint run ./...

# ── Clean ────────────────────────────────────────────────────────

clean:
	rm -rf bin/
	rm -rf sdk/typescript/dist
	rm -rf connectors/*/dist
