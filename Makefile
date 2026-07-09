# Codex Go Makefile
# Two modes: CLI (codex-go binary) and Desktop (Electron + Go backend + React UI)
# =============================================================================

# Build info
APP_NAME    := codex-go
VERSION     := 1.0.0
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)

# Cross-compile targets
GOOS_LINUX_AMD64   := GOOS=linux   GOARCH=amd64
GOOS_LINUX_ARM64   := GOOS=linux   GOARCH=arm64
GOOS_DARWIN_AMD64  := GOOS=darwin  GOARCH=amd64
GOOS_DARWIN_ARM64  := GOOS=darwin  GOARCH=arm64
GOOS_WINDOWS_AMD64 := GOOS=windows GOARCH=amd64

# Directories
DIST_DIR     := dist
CLI_DIR      := $(DIST_DIR)/cli
DESKTOP_DIR  := $(DIST_DIR)/desktop
WEB_DIR      := web
WEB_DIST     := $(WEB_DIR)/dist
DESKTOP_SRC  := desktop

.PHONY: all build build-all cli desktop web release clean install test

# Default: build CLI for current platform
all: build

# =============================================================================
# CLI Mode: single binary, no frontend bundled (uses --serve to self-host UI)
# =============================================================================

build: web
	@echo "🔨 Building CLI for current platform..."
	@mkdir -p $(CLI_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(CLI_DIR)/$(APP_NAME) ./cmd/codex/
	@echo "✅ CLI binary: $(CLI_DIR)/$(APP_NAME)"

build-all:
	@echo "🔨 Building CLI for all platforms..."
	@mkdir -p $(CLI_DIR)
	$(GOOS_LINUX_AMD64)   CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(CLI_DIR)/$(APP_NAME)-linux-amd64       ./cmd/codex/
	$(GOOS_LINUX_ARM64)   CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(CLI_DIR)/$(APP_NAME)-linux-arm64       ./cmd/codex/
	$(GOOS_DARWIN_AMD64)  CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(CLI_DIR)/$(APP_NAME)-darwin-amd64      ./cmd/codex/
	$(GOOS_DARWIN_ARM64)  CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(CLI_DIR)/$(APP_NAME)-darwin-arm64      ./cmd/codex/
	$(GOOS_WINDOWS_AMD64) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(CLI_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/codex/
	@echo ""
	@echo "✅ CLI binaries built:"
	@ls -lh $(CLI_DIR)/
	@echo ""
	@echo "📦 Creating checksums..."
	@cd $(CLI_DIR) && sha256sum * > checksums.txt
	@echo "✅ Done: $(CLI_DIR)/"

# =============================================================================
# Desktop Mode: electron-builder bundles Go backend + React UI + Electron shell
# =============================================================================

desktop: build-all
	@echo "🔨 Packaging desktop app..."
	@mkdir -p $(DESKTOP_DIR)
	@cp $(CLI_DIR)/$(APP_NAME)-linux-amd64 $(DESKTOP_DIR)/$(APP_NAME) 2>/dev/null || true
	@cp $(CLI_DIR)/$(APP_NAME)-darwin-amd64 $(DESKTOP_DIR)/$(APP_NAME) 2>/dev/null || true
	@# Copy web dist
	@cp -r $(WEB_DIST) $(DESKTOP_DIR)/web-dist
	@# Copy Electron app
	@cp -r $(DESKTOP_SRC) $(DESKTOP_DIR)/electron
	@echo "✅ Desktop package prepared: $(DESKTOP_DIR)/"
	@echo "   To build installer, run: cd $(DESKTOP_DIR)/electron && npm run build"

# =============================================================================
# Web Frontend
# =============================================================================

web:
	@echo "🔨 Building React frontend..."
	@cd $(WEB_DIR) && npm install --silent 2>/dev/null
	@cd $(WEB_DIR) && npx vite build --logLevel warn
	@echo "✅ Frontend built: $(WEB_DIST)/"

# =============================================================================
# Release: build everything + create archives
# =============================================================================

release: clean build-all web
	@echo "📦 Creating release archives..."
	@cd $(CLI_DIR) && for f in $(APP_NAME)-*; do \
		if [ -f "$$f" ]; then \
			tar -czf "$$f.tar.gz" "$$f" && echo "   $$f.tar.gz"; \
		fi; \
	done
	@echo ""
	@echo "🎉 Release ready: $(CLI_DIR)/"
	@echo "   Desktop app: run 'make desktop' then electron-builder"

# =============================================================================
# Utilities
# =============================================================================

clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(DIST_DIR)
	@rm -f $(APP_NAME)
	@cd $(WEB_DIR) && rm -rf dist node_modules 2>/dev/null || true
	@cd $(DESKTOP_SRC) && rm -rf node_modules 2>/dev/null || true
	@echo "✅ Clean"

install: build
	@echo "📦 Installing to /usr/local/bin/$(APP_NAME)..."
	@sudo cp $(CLI_DIR)/$(APP_NAME) /usr/local/bin/$(APP_NAME)
	@echo "✅ Installed: $(shell which $(APP_NAME))"

test:
	@echo "🧪 Running tests..."
	@go vet ./...
	@cd $(WEB_DIR) && npx tsc --noEmit
	@echo "✅ All checks passed"

# Quick dev targets
dev-cli:
	@go run ./cmd/codex/

dev-server:
	@go run ./cmd/codex/ --serve --addr :1977

dev-web:
	@cd $(WEB_DIR) && npm run dev

dev-desktop:
	@cd $(DESKTOP_SRC) && npm start

help:
	@echo "Codex Go Makefile"
	@echo ""
	@echo "CLI Mode (single binary):"
	@echo "  make build        Build CLI for current platform"
	@echo "  make build-all    Cross-compile CLI for all platforms"
	@echo "  make install      Install CLI to /usr/local/bin"
	@echo "  make cli          = make build"
	@echo ""
	@echo "Desktop Mode (Electron app):"
	@echo "  make desktop      Package desktop app"
	@echo "  make dev-desktop  Run desktop app in dev mode"
	@echo ""
	@echo "Web Frontend:"
	@echo "  make web          Build React frontend"
	@echo "  make dev-web      Run Vite dev server"
	@echo ""
	@echo "Release:"
	@echo "  make release      Build all platforms + create archives"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean        Remove all build artifacts"
	@echo "  make test         Run vet + tsc checks"
	@echo "  make dev-cli      Run CLI in dev mode"
	@echo "  make dev-server   Run API server in dev mode"
