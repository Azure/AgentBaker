GO              ?= go
LOCALBIN		= $(shell pwd)/bin

all: install

.PHONY: install
install: $(LOCALBIN)/go-bindata $(LOCALBIN)/gox $(LOCALBIN)/ginkgo $(LOCALBIN)/golangci-lint $(LOCALBIN)/cue
	@echo > /dev/null

$(LOCALBIN)/go-bindata:
	GOBIN=$(LOCALBIN) $(GO) get github.com/go-bindata/go-bindata/...@v3.1.2

$(LOCALBIN)/gox:
	GOBIN=$(LOCALBIN) $(GO) get github.com/mitchellh/gox/...@v1.0.1

$(LOCALBIN)/ginkgo:
	GOBIN=$(LOCALBIN) $(GO) get github.com/onsi/ginkgo/v2/ginkgo@v2.1.1

$(LOCALBIN)/golangci-lint:
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(LOCALBIN) v1.21.0

$(LOCALBIN)/cue:
	./scripts/setup_cue.sh

.PHONY: reload
reload: clean install

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)
