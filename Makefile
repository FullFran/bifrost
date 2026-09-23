PREFIX ?= $(HOME)/.local/bin
BINARY_NAME = bifrost

.PHONY: all build install uninstall clean test realms

all: build

build:
	@echo "🌈 Building Bifrost..."
	@mkdir -p bin
	go build -ldflags="-s -w" -o bin/$(BINARY_NAME) ./cmd/bifrost
	@echo "✅ Built bin/$(BINARY_NAME)"

install: build
	@echo "📦 Installing to $(PREFIX)..."
	@mkdir -p $(PREFIX)
	@cp bin/$(BINARY_NAME) $(PREFIX)/$(BINARY_NAME)
	@chmod +x $(PREFIX)/$(BINARY_NAME)
	@echo "✅ Installed $(PREFIX)/$(BINARY_NAME)"
	@echo "Run 'bifrost realms' to verify your agent connections."

uninstall:
	@echo "🗑️ Removing $(PREFIX)/$(BINARY_NAME)..."
	@rm -f $(PREFIX)/$(BINARY_NAME)
	@echo "Done."

clean:
	@rm -rf bin/

realms: build
	@./bin/$(BINARY_NAME) realms
