# WhatsApp-Go Makefile
# Provides common development tasks

.PHONY: help test test-coverage test-integration bench lint fmt vet tidy build clean install tools

# Default target
help:
	@echo "WhatsApp-Go Development Commands"
	@echo ""
	@echo "Testing:"
	@echo "  make test              - Run all unit tests"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo "  make test-integration  - Run integration tests (requires paired device)"
	@echo "  make test-race         - Run tests with race detector"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint              - Run golangci-lint"
	@echo "  make fmt               - Format code with gofmt"
	@echo "  make vet               - Run go vet"
	@echo "  make tidy              - Run go mod tidy"
	@echo ""
	@echo "Build:"
	@echo "  make build             - Build all packages"
	@echo "  make build-examples    - Build example applications"
	@echo "  make clean             - Clean build artifacts"
	@echo ""
	@echo "Benchmarks:"
	@echo "  make bench             - Run benchmarks"
	@echo "  make bench-cpu         - Run CPU benchmarks"
	@echo "  make bench-mem         - Run memory benchmarks"
	@echo ""
	@echo "Development:"
	@echo "  make install-tools     - Install development tools"
	@echo "  make generate          - Run go generate"
	@echo "  make deps              - Download dependencies"
	@echo ""

# Testing
test:
	go test ./... -v -count=1

test-coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-race:
	go test ./... -race -count=1

test-integration:
	@echo "Integration tests require a paired WhatsApp device"
	go test -tags=integration ./... -v -count=1

# Code Quality
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

tidy:
	go mod tidy
	go mod verify

# Build
build:
	go build ./...

build-examples:
	go build ./examples/...

# Benchmarks
bench:
	go test ./... -bench=. -benchmem -benchtime=3s

bench-cpu:
	go test ./... -bench=. -benchmem -benchtime=5s -cpuprofile=cpu.prof

bench-mem:
	go test ./... -bench=. -benchmem -benchtime=5s -memprofile=mem.prof

# Development
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

generate:
	go generate ./...

deps:
	go mod download

# Clean
clean:
	go clean ./...
	rm -f coverage.out coverage.html cpu.prof mem.prof
	rm -f whatsapp.db whatsapp.db-wal whatsapp.db-shm

# Security
security:
	gosec ./...
	govulncheck ./...

# Release
release-dry-run:
	goreleaser release --snapshot --clean --skip=publish

# Documentation
docs:
	@which godoc > /dev/null || go install golang.org/x/tools/cmd/godoc@latest
	@echo "Starting godoc server at http://localhost:6060"
	@echo "Press Ctrl+C to stop"
	godoc -http=:6060

# Check for common issues
check: fmt vet lint test
	@echo "All checks passed!"