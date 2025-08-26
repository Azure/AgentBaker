GO              ?= go
LOCALBIN		= $(shell pwd)/bin

all: install

.PHONY: install
install: $(LOCALBIN)/gox $(LOCALBIN)/ginkgo $(LOCALBIN)/golangci-lint $(LOCALBIN)/cue $(LOCALBIN)/oras
	@echo > /dev/null

$(LOCALBIN)/gox:
	GOBIN=$(LOCALBIN) $(GO) install github.com/mitchellh/gox/...@v1.0.1

$(LOCALBIN)/ginkgo:
	GOBIN=$(LOCALBIN) $(GO) install github.com/onsi/ginkgo/v2/ginkgo@v2.1.1

$(LOCALBIN)/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) v1.51.2

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
