TARGETS           = darwin/amd64 linux/amd64 windows/amd64
DIST_DIRS         = find * -type d -exec

.NOTPARALLEL:

.PHONY: bootstrap build test test_fmt validate-copyright-headers fmt lint ci

ifdef DEBUG
GOFLAGS   := -gcflags="-N -l"
else
GOFLAGS   :=
endif

# go option
GO              ?= go
TAGS            :=
LDFLAGS         :=
BINDIR          := $(CURDIR)/bin
PROJECT         := baker
VERSION         ?= $(shell git rev-parse HEAD)
VERSION_SHORT   ?= $(shell git rev-parse --short HEAD)
GITTAG          := $(shell git describe --exact-match --tags $(shell git log -n1 --pretty='%h') 2> /dev/null)
GOBIN			?= $(shell $(GO) env GOPATH)/bin
TOOLSBIN		:= $(CURDIR)/hack/tools/bin
ifeq ($(GITTAG),)
GITTAG := $(VERSION_SHORT)
endif

DEV_ENV_IMAGE := mcr.microsoft.com/oss/azcu/go-dev:v1.34.7
DEV_ENV_WORK_DIR := /baker
DEV_ENV_OPTS := --rm -v $(GOPATH)/pkg/mod:/go/pkg/mod -v $(CURDIR):$(DEV_ENV_WORK_DIR) -w $(DEV_ENV_WORK_DIR) $(DEV_ENV_VARS)
DEV_ENV_CMD := docker run $(DEV_ENV_OPTS) $(DEV_ENV_IMAGE)
DEV_ENV_CMD_IT := docker run -it $(DEV_ENV_OPTS) $(DEV_ENV_IMAGE)
DEV_CMD_RUN := docker run $(DEV_ENV_OPTS)
ifdef DEBUG
LDFLAGS := -X main.version=$(VERSION)
else
LDFLAGS := -s -X main.version=$(VERSION)
endif
BINARY_DEST_DIR ?= bin

ifeq ($(OS),Windows_NT)
	EXTENSION = .exe
	SHELL     = cmd.exe
	CHECK     = where.exe
else
	EXTENSION =
	SHELL     = bash
	CHECK     = which
endif

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# Add the tools bin to the front of the path
export PATH := $(TOOLSBIN):$(PATH)

all: generate

.PHONY: dev
dev:
	$(DEV_ENV_CMD_IT) bash

.PHONY: validate-dependencies
validate-dependencies: bootstrap
	@./scripts/validate-dependencies.sh

.PHONY: validate-copyright-headers
validate-copyright-headers:
	@./scripts/validate-copyright-header.sh

.PHONY: validate-go
validate-go:
	@./scripts/validate-go.sh

.PHONY: validate-shell
validate-shell:
	@./.pipelines/scripts/verify_shell.sh

.PHONY: shellspec-base-local
shellspec-base-local:
	# don't have access to aksdataplanedev.azurecr.io locally, so strip it
	sed 's|aksdataplanedev.azurecr.io/||g' < ./spec/shellspec.Dockerfile | docker build -t shellspec-docker -

.PHONY: shellspec-base-ci
shellspec-base-ci:
	# don't have access to aksdataplanedev.azurecr.io in CI, so strip it
	sed 's|aksdataplanedev.azurecr.io/||g' < ./spec/shellspec.Dockerfile | docker build -t shellspec-docker -
	# docker build -t shellspec-docker - < ./spec/shellspec.Dockerfile

.PHONY: shellspec
shellspec: shellspec-base-local
	docker run --rm \
		-v $(CURDIR):/workspace \
		-w /workspace \
		shellspec-docker --shell bash --format d

.PHONY: shellspec-ci
shellspec-ci: shellspec-base-ci
	docker run --rm \
		-v $(CURDIR):/workspace \
		-w /workspace \
		shellspec-docker --shell bash --format d

.PHONY: shellspec-focus
shellspec-focus: shellspec-base-local
	docker run --rm \
		-v $(CURDIR):/workspace \
		-w /workspace \
		shellspec-docker --shell bash --format d --focus

.PHONY: validate-image-version
validate-image-version:
	@./vhdbuilder/packer/test/run-pretest.sh

.PHONY: generate-kubelet-flags
generate-kubelet-flags:
	@./e2e/kubelet/generate-kubelet-flags.sh

.PHONY: generate-manifest
generate-manifest:
	./hack/tools/bin/cue export ./schemas/manifest.cue > ./parts/linux/cloud-init/artifacts/manifest.json
	@echo "#EOF" >> ./parts/linux/cloud-init/artifacts/manifest.json

.PHONY: generate-testdata
generate-testdata:
	@echo $(GOFLAGS)
	GENERATE_TEST_DATA="true" go test ./pkg/agent...

.PHONY: generate # TODO: ONLY generate go testdata
generate: bootstrap
	@echo "Generating go testdata"
	@$(MAKE) generate-testdata
	@echo "Generating manifest.cue"
	@$(MAKE) generate-manifest
	@echo "Running validate-shell to make sure generated cse scripts are correct"
	@$(MAKE) validate-shell
	@echo "Validating components.json conforms to the schema schemas/components.cue."
	@$(MAKE) validate-components

.PHONY: validate-prefetch
validate-prefetch:
	make -C ./vhdbuilder/prefetch generate

.PHONY: generate-azure-constants
generate-azure-constants:
	python pkg/helpers/generate_azure_constants.py

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: download
download: tidy
	$(GO) mod download

build-binary: generate
	go build $(GOFLAGS) -v -ldflags "$(LDFLAGS)" -o $(BINARY_DEST_DIR)/baker .

# usage: make clean build-cross dist VERSION=v0.4.0
.PHONY: build-cross
build-cross: build
build-cross: LDFLAGS += -extldflags "-static"
build-cross:
	CGO_ENABLED=0 gox -output="_dist/baker-$(GITTAG)-{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)'

.PHONY: dist
dist: build-cross compress-binaries
	( \
		cd _dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) tar -zcf {}.tar.gz {} \; && \
		$(DIST_DIRS) zip -r {}.zip {} \; \
	)

.PHONY: compress-binaries
compress-binaries:
	@which upx || (echo "Please install the upx executable packer tool. See https://upx.github.io/" && exit 1)
	find _dist -type f \( -name "baker" -o -name "baker.exe" \) -exec upx -9 {} +

.PHONY: checksum
checksum:
	for f in _dist/*.{gz,zip} ; do \
		shasum -a 256 "$${f}"  | awk '{print $$1}' > "$${f}.sha256" ; \
	done

.PHONY: build-container
build-container:
	docker build --no-cache --build-arg BUILD_DATE=`date -u +"%Y-%m-%dT%H:%M:%SZ"` \
		--build-arg AGENTBAKER_VERSION="$(VERSION)" -t microsoft/baker:$(VERSION) \
		--file ./releases/Dockerfile.linux ./releases || \
	echo 'This target works only for published releases. For example, "VERSION=0.32.0 make build-container".'

.PHONY: clean
clean: tools-clean
	@rm -rf $(BINDIR) ./_dist ./pkg/helpers/unit_tests

GIT_BASEDIR    = $(shell git rev-parse --show-toplevel 2>/dev/null)
ifneq ($(GIT_BASEDIR),)
	LDFLAGS += -X github.com/Azure/agentbaker/pkg/test.JUnitOutDir=$(GIT_BASEDIR)/test/junit
endif

ginkgoBuild: generate
	make -C ./test/e2e ginkgo-build

test: test-aks-node-controller
	go build -mod=readonly ./... && go test ./...

test-aks-node-controller:
	pushd aks-node-controller && go build -mod=readonly ./... && go test ./... && popd

.PHONY: test-style
test-style: validate-go validate-shell validate-copyright-headers

.PHONY: ensure-generated
ensure-generated:
	@echo "==> Checking generated files <=="
	@scripts/ensure-generated.sh

.PHONY: test-e2e
test-e2e:
	@test/e2e.sh

HAS_GIT := $(shell $(CHECK) git)

.PHONY: bootstrap
bootstrap: tools-install
ifndef HAS_GIT
	$(error You must install Git)
endif

.PHONY: tools-reload
tools-reload:
	make -C hack/tools reload

.PHONY: tools-install
tools-install:
	make -C hack/tools/

.PHONY: tools-clean
tools-clean:
	make -C hack/tools/ clean

ci: bootstrap test-style build test lint
	./scripts/coverage.sh --coveralls

.PHONY: coverage
coverage:
	@scripts/ginkgo.coverage.sh --codecov

.PHONY: unit-tests
unit-tests:
	$(GO) test `go list ./... | grep -v e2e` -coverprofile coverage_raw.out -covermode count

.PHONY: validate-components
validate-components:
	@./hack/tools/bin/cue vet -c ./schemas/components.cue ./parts/common/components.json
	@./hack/tools/bin/cue vet -c ./schemas/components.cue ./vhdbuilder/packer/windows/components-test.json
	@./hack/tools/bin/cue vet -c ./schemas/windows_settings.cue ./vhdbuilder/packer/windows/windows_settings.json

include versioning.mk
