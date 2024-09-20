SHELL = bash

.PHONY: all

all: generate

.PHONY: generate
generate: generate-fixtures
	PREFETCH_REGENERATE_FIXTURES="true" go test ./...

.PHONY: test
test:
	go test ./...

.PHONY:
generate-fixtures:
	@bash ./hack/generate-fixtures.sh