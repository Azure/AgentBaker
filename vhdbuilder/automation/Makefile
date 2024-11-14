# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Defaults
GOARCH=amd64
GOOS=linux

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o bin/automation-$(GOARCH) main.go

.PHONY: build-all
build-all:
	@$(MAKE) build GOOS=linux ARCH=amd64
	@$(MAKE) build GOOS=linux ARCH=arm64

.PHONY: test
test:
	go test ./...