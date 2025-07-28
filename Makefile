# LiveKit Agents Go - Build and Run Helpers

# Phase 1 targets as per specification
.PHONY: test lint vet
test:
	go test ./... -race -coverprofile=coverage.out

lint:
	golangci-lint run

vet:
	go vet ./...

# Build CLI
.PHONY: build
build:
	mkdir -p bin
	go build -o bin/lk-go ./cmd/lk-go

# Build with version info
.PHONY: build-with-version
build-with-version:
	mkdir -p bin
	go build -ldflags "-X github.com/chriscow/livekit-agents-go/pkg/version.Version=$(shell git describe --tags --always --dirty) -X github.com/chriscow/livekit-agents-go/pkg/version.GitCommit=$(shell git rev-parse HEAD) -X github.com/chriscow/livekit-agents-go/pkg/version.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/lk-go ./cmd/lk-go

# Run CLI commands for testing
.PHONY: run-version run-dry-run run-healthz
run-version:
	go run ./cmd/lk-go version

run-dry-run:
	go run ./cmd/lk-go worker run --dry-run

run-healthz:
	go run ./cmd/lk-go worker healthz

# Legacy targets for existing examples (if they exist)
.PHONY: run-basic-console run-basic-dev run-basic-start
run-basic-console:
	@if [ -d "./examples/basic-agent" ]; then go run ./examples/basic-agent console; else echo "examples/basic-agent not found"; fi

run-basic-dev:
	@if [ -d "./examples/basic-agent" ]; then go run ./examples/basic-agent dev; else echo "examples/basic-agent not found"; fi

run-basic-start:
	@if [ -d "./examples/basic-agent" ]; then go run ./examples/basic-agent start; else echo "examples/basic-agent not found"; fi

# Build examples (if they exist)
.PHONY: build-examples
build-examples:
	@if [ -d "./examples/basic-agent" ]; then mkdir -p bin && go build -o bin/basic-agent ./examples/basic-agent; else echo "examples/basic-agent not found"; fi

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/ coverage.out lk-go

.PHONY: help
help:
	@echo "Available targets:"
	@echo "Phase 1 Core Foundation:"
	@echo "  test              - Run all tests with race detection and coverage"
	@echo "  lint              - Run golangci-lint"
	@echo "  vet               - Run go vet"
	@echo "  build             - Build lk-go CLI binary"
	@echo "  build-with-version - Build with git version info"
	@echo "  run-version       - Test version command"
	@echo "  run-dry-run       - Test worker dry-run"
	@echo "  run-healthz       - Test health check"
	@echo ""
	@echo "Legacy targets:"
	@echo "  run-basic-console  - Run basic agent in console mode (if exists)"
	@echo "  run-basic-dev      - Run basic agent in dev mode (if exists)"
	@echo "  run-basic-start    - Run basic agent in production mode (if exists)"
	@echo "  build-examples     - Build all examples (if they exist)"
	@echo "  clean              - Clean build artifacts"