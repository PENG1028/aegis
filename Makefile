# Aegis — Infrastructure Gateway Control Plane
# Build system

.PHONY: all build build-linux build-windows test test-race vet clean release dev-ui

# ─── Variables ───
BINARY      = aegis
GO          = go
GOFLAGS     = -ldflags="-s -w"
MAIN        = ./cmd/aegis/
UI_DIR      = ./ui
UI_DIST     = $(UI_DIR)/dist
UI_EMBED    = ./internal/uiassets/dist
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME  = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# ─── Default target ───
all: build

# ─── Build ───
build: build-ui embed-ui
	$(GO) build $(GOFLAGS) -o $(BINARY) $(MAIN)

build-linux: build-ui embed-ui
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY) $(MAIN)

build-windows: build-ui embed-ui
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY).exe $(MAIN)

# Release build with version injected
release: build-ui embed-ui
	GOOS=linux GOARCH=amd64 $(GO) build \
		-ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)" \
		-o $(BINARY) $(MAIN)

# ─── UI ───
dev-ui:
	cd $(UI_DIR) && npm run dev

build-ui:
	cd $(UI_DIR) && npm run build

embed-ui:
	@echo "  Embedding UI dist..."
	@rm -rf $(UI_EMBED)
	@cp -r $(UI_DIST) $(UI_EMBED)
	@echo "  UI dist embedded ($(UI_EMBED))"

# ─── Test ───
test:
	$(GO) test ./... -count=1 -timeout=120s

test-race:
	$(GO) test -race ./... -count=1 -timeout=120s

test-cover:
	$(GO) test ./... -coverprofile=coverage.out -count=1 -timeout=120s
	$(GO) tool cover -html=coverage.out -o coverage.html

# ─── Quality ───
vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint: vet
	@echo "All checks passed"

# ─── Clean ───
clean:
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage.html
	rm -rf $(UI_DIR)/dist $(UI_EMBED)
