VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := harbor
GOFLAGS := -trimpath -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build clean test lint install gateway sdk-build connector-build

# ── Build ────────────────────────────────────────────────────────

build:
	go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/harbor

gateway:
	go build $(GOFLAGS) -o bin/harbor-gateway ./gateway

install: build
	cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || \
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)

# ── SDK & Connectors ─────────────────────────────────────────────

sdk-build:
	cd sdk/typescript && npm install && npm run build

connector-build: sdk-build
	cd connectors/coingecko && npm install && npm run build

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
