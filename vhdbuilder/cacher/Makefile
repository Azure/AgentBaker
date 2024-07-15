# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Default arch is amd64.
ARCH=amd64

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/cacher cmd/cacher/main.go