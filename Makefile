# Sosomi Makefile

BINARY_NAME=sosomi
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build install clean test lint run help release-dry-run release-snapshot

all: build

## Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/sosomi

## Build for all platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/sosomi
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/sosomi
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/sosomi
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/sosomi

## Install to ~/.local/bin
install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BINARY_NAME) $(HOME)/.local/bin/
	@echo "Installed to $(HOME)/.local/bin/$(BINARY_NAME)"
	@echo "Make sure $(HOME)/.local/bin is in your PATH"

## Install system-wide (requires sudo)
install-system: build
	sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"

## Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	go clean

## Run tests
test:
	go test -v ./...

## Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## Run linter
lint:
	golangci-lint run ./...

## Run the application
run:
	go run ./cmd/sosomi $(ARGS)

## Update dependencies
deps:
	go mod tidy
	go mod download

## Generate shell completions
completions:
	mkdir -p completions
	./$(BINARY_NAME) completion bash > completions/$(BINARY_NAME).bash
	./$(BINARY_NAME) completion zsh > completions/_$(BINARY_NAME)
	./$(BINARY_NAME) completion fish > completions/$(BINARY_NAME).fish
	@echo "Shell completions generated in completions/"

## Install shell integration
install-shell-integration:
	@echo "For zsh, add to ~/.zshrc:"
	@echo "  source $(PWD)/scripts/zsh-integration.zsh"
	@echo ""
	@echo "For bash, add to ~/.bashrc:"
	@echo "  source $(PWD)/scripts/bash-integration.bash"

## Show help
help:
	@echo "Sosomi Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
	@echo ""
	@echo "Examples:"
	@echo "  make build          # Build the binary"
	@echo "  make install        # Install to ~/.local/bin"
	@echo "  make run ARGS='\"list files\"'  # Run with arguments"

## Test GoReleaser locally (dry run)
release-dry-run:
	goreleaser release --snapshot --clean --skip=publish

## Create a snapshot release (for testing)
release-snapshot:
	goreleaser release --snapshot --clean
