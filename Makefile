.PHONY: build build-tools run chat serve test clean

# Build the main CLI binary
build:
	go build -o bin/forge ./cmd/forge

# Build individual tool servers
build-tool-shell-exec:
	go build -o bin/forge-tool-shell-exec ./cmd/tools/shell-exec

build-tool-file-ops:
	go build -o bin/forge-tool-file-ops ./cmd/tools/file-ops

build-tool-web-search:
	go build -o bin/forge-tool-web-search ./cmd/tools/web-search

build-tool-github-ops:
	go build -o bin/forge-tool-github-ops ./cmd/tools/github-ops

build-tool-code-runner:
	go build -o bin/forge-tool-code-runner ./cmd/tools/code-runner

# Build all tool server binaries
build-tools: build-tool-shell-exec build-tool-file-ops build-tool-web-search build-tool-github-ops build-tool-code-runner

# Build everything
all: build build-tools

# Interactive chat
chat: all
	./bin/forge chat

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
