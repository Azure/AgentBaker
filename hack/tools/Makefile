GO              ?= go
LOCALBIN		= $(shell pwd)/bin

all: install

.PHONY: install
install: $(LOCALBIN)/go-bindata $(LOCALBIN)/gox $(LOCALBIN)/ginkgo $(LOCALBIN)/golangci-lint $(LOCALBIN)/cue $(LOCALBIN)/buf
	@echo > /dev/null

$(LOCALBIN)/go-bindata:
	GOBIN=$(LOCALBIN) $(GO) install github.com/go-bindata/go-bindata/...@v3.1.2

$(LOCALBIN)/gox:
	GOBIN=$(LOCALBIN) $(GO) install github.com/mitchellh/gox/...@v1.0.1

$(LOCALBIN)/ginkgo:
	GOBIN=$(LOCALBIN) $(GO) install github.com/onsi/ginkgo/v2/ginkgo@v2.1.1

$(LOCALBIN)/golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) v1.51.2

$(LOCALBIN)/cue:
	GOBIN=$(LOCALBIN) $(GO) install cuelang.org/go/cmd/cue@v0.4.2

$(LOCALBIN)/buf:
	curl -sSL "https://github.com/bufbuild/buf/releases/download/v1.26.1/buf-$(shell uname -s)-$(shell uname -m)" -o $(LOCALBIN)/buf && \
	chmod +x $(LOCALBIN)/buf

.PHONY: reload
reload: clean install

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)
