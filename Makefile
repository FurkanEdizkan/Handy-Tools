SHELL := /usr/bin/env bash

MODULE      := github.com/furkandedizkan/handy-tools
BIN_DIR     := bin
TUI_BIN     := $(BIN_DIR)/htools
SERVER_BIN  := $(BIN_DIR)/htoolsd

GO          ?= go
GOFLAGS     ?=
LDFLAGS     ?= -s -w
PKGS        := ./...

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: web $(TUI_BIN) $(SERVER_BIN) ## Build web assets + both binaries

$(TUI_BIN):
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(TUI_BIN) ./cmd/htools

$(SERVER_BIN):
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(SERVER_BIN) ./cmd/htoolsd

.PHONY: web
web: ## Build the Svelte frontend into web/dist (embedded by htoolsd)
	cd web && npm ci && npm run build
	@# vite build empties dist/ — re-create the marker so the directory
	@# remains tracked in git for downstream `go build` without Node.
	@touch web/dist/.gitkeep

.PHONY: web-dev
web-dev: ## Run Vite dev server with HMR (point it at a running htoolsd)
	cd web && npm run dev

.PHONY: extension
extension: ## Build the Chrome MV3 extension into extension/dist
	cd extension && npm ci && npm run build

.PHONY: tui
tui: ## Run the TUI
	$(GO) run ./cmd/htools

.PHONY: serve
serve: ## Run the gRPC server
	$(GO) run ./cmd/htoolsd

.PHONY: test
test: ## Run unit tests
	$(GO) test -race -count=1 $(PKGS)

.PHONY: cover
cover: ## Run tests with coverage report
	$(GO) test -race -count=1 -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -html=coverage.out -o coverage.html

.PHONY: fuzz
fuzz: ## Run short fuzz pass over the config YAML decoder
	$(GO) test -run=^$$ -fuzz=FuzzDecodeYAML -fuzztime=20s ./internal/config

.PHONY: lint
lint: ## Run linters
	golangci-lint run
	@if command -v buf >/dev/null 2>&1; then buf lint; else echo "buf not installed, skipping proto lint"; fi

.PHONY: proto
proto: ## Regenerate protobuf bindings
	buf generate

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) dist coverage.out coverage.html
	rm -rf web/dist/assets web/dist/index.html
	rm -rf extension/dist
	@# Keep web/dist/.gitkeep (committed) and web/placeholder.html.
