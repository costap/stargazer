# Variables
APP_NAME    := stargazer
BIN_DIR     := bin
BINARY      := $(BIN_DIR)/$(APP_NAME)
MAIN_PKG    := ./cmd/stargazer

# Go toolchain
GOCMD       := go
GOBUILD     := $(GOCMD) build
GOINSTALL   := $(GOCMD) install
GOTEST      := $(GOCMD) test
GOVET       := $(GOCMD) vet
GOFMT       := gofmt
GOMOD       := $(GOCMD) mod

# Git metadata for build stamping
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE        ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: help all build clean test install run

## help: Display this help message (default)
help:
	@echo "Stargazer Build Targets:"
	@grep -E '^## [a-zA-Z_-]+:' $(MAKEFILE_LIST) | sed -e 's/## //' | awk -F': ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## all: Format, vet, test, and build binary
all: fmt vet test build 

## build: Build the stargazer binary into bin/stargazer
build:
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN_PKG)

install:
	$(GOINSTALL) -ldflags "$(LDFLAGS)" $(MAIN_PKG)

test:
	$(GOTEST) -v ./...

run:
	$(GOCMD) run $(MAIN_PKG) $(ARGS)