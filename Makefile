.PHONY: build build-tools run chat serve test clean

# Build the main CLI binary
build:
	go build -o bin/forge ./cmd/forge

# Build all tool server binaries (Phase 2)
build-tools:
	@echo "Tool servers not yet implemented (Phase 2)"

# Build everything
all: build

# Interactive chat
chat: build
	./bin/forge chat

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
