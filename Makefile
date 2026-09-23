PREFIX ?= $(HOME)/.local/bin
BINARY_NAME = bifrost
MODULE = github.com/FullFran/bifrost

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.2.0")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w \
  -X $(MODULE)/internal/buildinfo.version=$(VERSION) \
  -X $(MODULE)/internal/buildinfo.commit=$(COMMIT) \
  -X $(MODULE)/internal/buildinfo.date=$(DATE)

.PHONY: all build install uninstall clean test lint ci snapshot realms fmt

all: build

build:
	@echo "🌈 Building Bifrost..."
	@mkdir -p bin
	go build -ldflags="$(LDFLAGS)" -trimpath -o bin/$(BINARY_NAME) ./cmd/bifrost
	@echo "✅ Built bin/$(BINARY_NAME)"

install: build
	@echo "📦 Installing to $(PREFIX)..."
	@mkdir -p $(PREFIX)
	@cp bin/$(BINARY_NAME) $(PREFIX)/$(BINARY_NAME)
	@chmod +x $(PREFIX)/$(BINARY_NAME)
	@echo "✅ Installed $(PREFIX)/$(BINARY_NAME)"
	@$(PREFIX)/$(BINARY_NAME) realms

uninstall:
	@echo "🗑️ Removing $(PREFIX)/$(BINARY_NAME)..."
	@rm -f $(PREFIX)/$(BINARY_NAME)
	@echo "Done."

fmt:
	@echo "🎨 Formatting Go files..."
	@gofmt -s -w .

test:
	@echo "🧪 Running unit tests with race detector..."
	go test -race ./...

lint:
	@echo "🔍 Running linter..."
	golangci-lint run

snapshot:
	@echo "📸 Building release snapshot with GoReleaser..."
	goreleaser release --snapshot --clean

ci: fmt test lint
	@echo "🔨 Validating GoReleaser config..."
	goreleaser check
	@echo "✅ All CI checks passed locally!"

clean:
	@rm -rf bin/ dist/

realms: build
	@./bin/$(BINARY_NAME) realms
