.PHONY: build build-tools run chat serve test clean install uninstall

PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin
CONFIGDIR = $(HOME)/.forge

TOOLS = shell-exec file-ops web-search github-ops code-runner

# Build the main CLI binary
build:
	go build -o bin/forge ./cmd/forge

# Build individual tool servers
build-tool-%:
	go build -o bin/forge-tool-$* ./cmd/tools/$*

# Build all tool server binaries
build-tools: $(addprefix build-tool-,$(TOOLS))

# Build everything
all: build build-tools

# Install binaries and config
install: all
	@echo "Installing binaries to $(BINDIR)..."
	install -d $(BINDIR)
	install bin/forge $(BINDIR)/forge
	@for tool in $(TOOLS); do \
		install bin/forge-tool-$$tool $(BINDIR)/forge-tool-$$tool; \
	done
	@echo "Setting up config in $(CONFIGDIR)..."
	install -d $(CONFIGDIR)
	@if [ ! -f $(CONFIGDIR)/forge.yaml ]; then \
		sed 's|binary: "bin/forge-tool-|binary: "forge-tool-|g' forge.yaml > $(CONFIGDIR)/forge.yaml; \
		echo "Created $(CONFIGDIR)/forge.yaml"; \
	else \
		echo "$(CONFIGDIR)/forge.yaml already exists, skipping"; \
	fi
	@echo "Done. Run 'forge chat' from any directory."

# Remove installed binaries
uninstall:
	rm -f $(BINDIR)/forge
	@for tool in $(TOOLS); do \
		rm -f $(BINDIR)/forge-tool-$$tool; \
	done
	@echo "Binaries removed. Config left in $(CONFIGDIR)."

# Interactive chat
chat: all
	./bin/forge chat

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
