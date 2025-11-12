SHELL = bash

.PHONY: all

all: generate

.PHONY: generate
generate:
	REGENERATE_CONTAINER_IMAGE_PREFETCH_TESTDATA="true" go test ./...

.PHONY: test
test:
	go test ./...