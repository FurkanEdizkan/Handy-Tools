SHELL := /usr/bin/env bash

MODULE      := github.com/furkandedizkan/handy
BIN_DIR     := bin
TUI_BIN     := $(BIN_DIR)/handy
SERVER_BIN  := $(BIN_DIR)/handyd

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
build: $(TUI_BIN) $(SERVER_BIN) ## Build both binaries

$(TUI_BIN):
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(TUI_BIN) ./cmd/handy

$(SERVER_BIN):
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(SERVER_BIN) ./cmd/handyd

.PHONY: tui
tui: ## Run the TUI
	$(GO) run ./cmd/handy

.PHONY: serve
serve: ## Run the gRPC server
	$(GO) run ./cmd/handyd

.PHONY: test
test: ## Run unit tests
	$(GO) test -race -count=1 $(PKGS)

.PHONY: cover
cover: ## Run tests with coverage report
	$(GO) test -race -count=1 -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -html=coverage.out -o coverage.html

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
