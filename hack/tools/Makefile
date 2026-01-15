GO              ?= go
LOCALBIN		= $(shell pwd)/bin

# Detect OS and architecture
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Linux)
    GOOS := linux
endif
ifeq ($(UNAME_S),Darwin)
    GOOS := darwin
endif

ifeq ($(UNAME_M),x86_64)
    GOARCH := amd64
endif
ifeq ($(UNAME_M),arm64)
    GOARCH := arm64
endif
ifeq ($(UNAME_M),aarch64)
    GOARCH := arm64
endif

GOLANGCI_LINT_VERSION := v2.8.0

all: install

.PHONY: install
install: $(LOCALBIN)/gox $(LOCALBIN)/ginkgo $(LOCALBIN)/golangci-lint $(LOCALBIN)/cue $(LOCALBIN)/oras
	@echo > /dev/null

$(LOCALBIN)/gox:
	GOBIN=$(LOCALBIN) $(GO) install github.com/mitchellh/gox/...@v1.0.1

$(LOCALBIN)/ginkgo:
	GOBIN=$(LOCALBIN) $(GO) install github.com/onsi/ginkgo/v2/ginkgo@v2.1.1

$(LOCALBIN)/golangci-lint:
	@mkdir -p $(LOCALBIN)
	curl -sSfL -o $(LOCALBIN)/golangci-lint.tar.gz \
		https://github.com/golangci/golangci-lint/releases/download/$(GOLANGCI_LINT_VERSION)/golangci-lint-$(subst v,,$(GOLANGCI_LINT_VERSION))-$(GOOS)-$(GOARCH).tar.gz
	tar -xzf $(LOCALBIN)/golangci-lint.tar.gz -C $(LOCALBIN) --strip-components=1 golangci-lint-$(subst v,,$(GOLANGCI_LINT_VERSION))-$(GOOS)-$(GOARCH)/golangci-lint
	rm -f $(LOCALBIN)/golangci-lint.tar.gz

$(LOCALBIN)/cue:
	GOBIN=$(LOCALBIN) $(GO) install cuelang.org/go/cmd/cue@v0.4.2

$(LOCALBIN)/butane:
	GOBIN=$(LOCALBIN) $(GO) install github.com/coreos/butane/internal@v0.24.0
	mv $(LOCALBIN)/internal $(LOCALBIN)/butane

$(LOCALBIN)/oras:
	GOBIN=$(LOCALBIN) $(GO) install oras.land/oras/cmd/oras@v1.2.3

.PHONY: reload
reload: clean install

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)
