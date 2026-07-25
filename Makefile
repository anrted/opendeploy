PKG := github.com/anrted/opendeploy
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X $(PKG)/pkg/version.Version=$(VERSION) -X $(PKG)/pkg/version.Commit=$(COMMIT) -X $(PKG)/pkg/version.BuildTime=$(BUILD_TIME) -s -w"

CORE_BIN   := bin/opendeploy-core
AGENT_BIN  := bin/opendeploy-agent
CLI_BIN    := bin/opendeploy-cli

.PHONY: all build check-build-deps frontend build-core build-agent build-cli clean test lint proto dev-core dev-agent install uninstall

.NOTPARALLEL: build install

all: build

## Build all binaries
build: check-build-deps frontend build-core build-agent build-cli
	@echo ""
	@echo "OpenDeploy build completed."
	@echo "Binaries are available in ./bin."
ifeq ($(filter install,$(MAKECMDGOALS)),)
	@echo "To install and start the panel, run: sudo make install"
endif

check-build-deps:
	sh deployments/check-build-deps.sh

frontend:
	cd web && npm ci && npm run build
	rm -rf internal/core/webui/dist
	mkdir -p internal/core/webui/dist
	cp -R web/dist/. internal/core/webui/dist/

build-core:
	@echo "→ Building OpenDeploy Core..."
	@mkdir -p bin
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(CORE_BIN) ./cmd/core

build-agent:
	@echo "→ Building OpenDeploy Agent..."
	@mkdir -p bin
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(AGENT_BIN) ./cmd/agent

build-cli:
	@echo "→ Building OpenDeploy CLI..."
	@mkdir -p bin
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(CLI_BIN) ./cmd/cli

## Development
dev-agent:
	@echo "→ Starting Agent (dev)..."
	go run ./cmd/agent --config configs/opendeploy.yaml

dev-core:
	@echo "→ Starting Core (dev)..."
	go run ./cmd/core --config configs/opendeploy.yaml

## Code generation
proto:
	@echo "→ Generating protobuf code..."
	protoc -I . --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/agent/v1/agent.proto

## Testing
test:
	go test -v -race -coverprofile=coverage.out ./...

test-short:
	go test -short ./...

coverage:
	go tool cover -html=coverage.out

## Code quality
lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

## Dependencies
deps:
	go mod download
	go mod tidy

## Cleanup
clean:
	rm -rf bin/
	rm -f coverage.out

## Install to system (Linux)
install: build
	sh deployments/install-dev.sh

uninstall:
	sh deployments/uninstall.sh

## Help
help:
	@echo "OpenDeploy Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make build        Build all binaries"
	@echo "                    Missing Ubuntu/Debian build packages are installed automatically"
	@echo "  make dev-core     Run Core in development mode"
	@echo "  make dev-agent    Run Agent in development mode"
	@echo "  make test         Run all tests"
	@echo "  make lint         Run linter"
	@echo "  make proto        Regenerate protobuf"
	@echo "  make install      Install to system (requires root)"
	@echo "  make clean        Remove build artifacts"
