# LiveKit Agents Go - Build and Run Helpers

# Run basic agent in different modes
.PHONY: run-basic-console run-basic-dev run-basic-start
run-basic-console:
	go run ./examples/basic-agent console

run-basic-dev:
	go run ./examples/basic-agent dev

run-basic-start:
	go run ./examples/basic-agent start

# Run tests
.PHONY: test
test:
	go test ./...

# Build examples
.PHONY: build-examples
build-examples:
	mkdir -p bin
	go build -o bin/basic-agent ./examples/basic-agent

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  run-basic-console  - Run basic agent in console mode"
	@echo "  run-basic-dev      - Run basic agent in dev mode"  
	@echo "  run-basic-start    - Run basic agent in production mode"
	@echo "  test              - Run all tests"
	@echo "  build-examples    - Build all examples"
	@echo "  clean             - Clean build artifacts"