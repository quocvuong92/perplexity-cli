.PHONY: build build-compressed build-darwin build-linux build-windows build-all build-all-compressed clean test fmt lint

BINARY_NAME=perplexity
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=build

# Build for current OS
build:
	go build -ldflags="-s -w" -o $(BINARY_NAME) .

# Build and compress for current OS
build-compressed: build
	gzexe $(BINARY_NAME)
	rm -f $(BINARY_NAME)~

# Build for macOS (both architectures)
build-darwin:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .

# Build for Linux
build-linux:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .

# Build for Windows
build-windows:
	mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Build for all platforms
build-all: build-darwin build-linux build-windows

# Build and compress for all platforms (gzexe doesn't work on Windows)
build-all-compressed: build-all
	gzexe $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
	gzexe $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	gzexe $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	rm -f $(BUILD_DIR)/*~
	@echo "Note: Windows binary not compressed (gzexe not supported)"

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)~
	rm -rf $(BUILD_DIR)

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run
