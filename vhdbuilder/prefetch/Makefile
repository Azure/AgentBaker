SHELL = bash

.PHONY: all

all: generate

.PHONY: generate
generate: generate-testdata
	REGENERATE_CONTAINER_IMAGE_PREFETCH_TESTDATA="true" go test ./...

.PHONY: test
test:
	go test ./...

.PHONY: generate-testdata
generate-testdata:
	@bash ./hack/generate-testdata.sh