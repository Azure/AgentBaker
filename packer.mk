SHELL=/bin/bash -o pipefail

GOARCH=amd64
ifeq (${ARCHITECTURE},ARM64)
	GOARCH=arm64
endif
GOHOSTARCH = $(shell go env GOHOSTARCH)

build-packer: setup-golang generate-prefetch-scripts build-image-fetcher build-aks-node-controller build-lister-binary
ifeq (${ARCHITECTURE},ARM64)
	@echo "${MODE}: Building with Hyper-v generation 2 ARM64 VM"
ifeq (${OS_SKU},Ubuntu)
ifeq ($(findstring NVIDIA_GB,$(FEATURE_FLAGS)),NVIDIA_GB)
	@echo "Using packer template file vhd-image-builder-arm64-gb.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-arm64-gb.json
else
	@echo "Using packer template file vhd-image-builder-arm64-gen2.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
endif
else ifeq (${OS_SKU},CBLMariner)
	@echo "Using packer template file vhd-image-builder-mariner-arm64.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
else ifeq (${OS_SKU},AzureLinux)
	@echo "Using packer template file vhd-image-builder-mariner-arm64.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
else ifeq (${OS_SKU},AzureContainerLinux)
	@echo "Using packer template file vhd-image-builder-acl-arm64.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-acl-arm64.json
else
	$(error OS_SKU was invalid ${OS_SKU})
endif
else ifeq (${ARCHITECTURE},X86_64)
ifeq (${HYPERV_GENERATION},V2)
	@echo "${MODE}: Building with Hyper-v generation 2 x86_64 VM"
else ifeq (${HYPERV_GENERATION},V1)
	@echo "${MODE}: Building with Hyper-v generation 1 X86_64 VM"
else
	$(error HYPERV_GENERATION was invalid ${HYPERV_GENERATION})
endif
ifeq (${OS_SKU},Ubuntu)
ifeq ($(findstring cvm,$(FEATURE_FLAGS)),cvm)
	@echo "Using packer template file vhd-image-builder-cvm.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-cvm.json
else
	@echo "Using packer template file vhd-image-builder-base.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-base.json
endif
else ifeq (${OS_SKU},CBLMariner)
	@echo "Using packer template file vhd-image-builder-mariner.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner.json
else ifeq (${OS_SKU},AzureLinux)
ifeq ($(findstring cvm,$(FEATURE_FLAGS)),cvm)
	@echo "Using packer template file vhd-image-builder-mariner-cvm.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-cvm.json
else
	@echo "Using packer template file vhd-image-builder-mariner.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner.json
endif
else ifeq (${OS_SKU},AzureContainerLinux)
	@echo "Using packer template file vhd-image-builder-acl.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-acl.json
else
	$(error OS_SKU was invalid ${OS_SKU})
endif
endif

build-packer-windows:
ifeq (${MODE},windowsVhdMode)
ifeq (${SIG_FOR_PRODUCTION},True)
ifeq (${HYPERV_GENERATION},V1)
	@echo "${MODE}: Building with Hyper-v generation 1 VM and save to Classic Storage Account"
else
	@echo "${MODE}: Building with Hyper-v generation 2 VM and save to Classic Storage Account"
endif
else
ifeq (${HYPERV_GENERATION},V1)
	@echo "${MODE}: Building with Hyper-v generation 1 VM and save to Shared Image Gallery"
else
	@echo "${MODE}: Building with Hyper-v generation 2 VM and save to Shared Image Gallery"
endif
endif
	@packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows/windows-vhd-builder-sig.json
endif

build-imagecustomizer: setup-golang generate-prefetch-scripts build-image-fetcher build-aks-node-controller build-lister-binary
	@./vhdbuilder/packer/imagecustomizer/scripts/build-imagecustomizer-image.sh

az-login:
	@echo "Using the subscription ${SUBSCRIPTION_ID}"
	@az account set -s ${SUBSCRIPTION_ID}

init-packer:
	@./vhdbuilder/packer/produce-packer-settings.sh

run-packer: az-login
	@packer init ./vhdbuilder/packer/packer-plugin.pkr.hcl && packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer | tee -a packer-output)

run-imagecustomizer: az-login
	@($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-imagecustomizer | tee -a packer-output)

CVM_BOOTSTRAP_PACKER_OUTPUT := packer-output-bootstrap
CVM_FINAL_TEMPLATE := vhdbuilder/packer/vhd-image-builder-cvm-2604.json

validate-cvm-two-stage:
	@test "$(CVM_TWO_STAGE_BUILD)" = "True" || { echo "CVM two-stage builds require CVM_TWO_STAGE_BUILD=True"; exit 1; }
	@test "$(OS_SKU)" = "Ubuntu" || { echo "CVM two-stage builds require OS_SKU=Ubuntu"; exit 1; }
	@test "$(OS_VERSION)" = "26.04" || { echo "CVM two-stage builds require OS_VERSION=26.04"; exit 1; }
	@case "$(FEATURE_FLAGS)" in *cvm*) ;; *) echo "CVM two-stage builds require FEATURE_FLAGS to contain cvm"; exit 1 ;; esac

validate-cvm-final: validate-cvm-two-stage
	@test -n "$(CVM_BOOTSTRAP_SUBSCRIPTION_ID)" || { echo "CVM final build requires CVM_BOOTSTRAP_SUBSCRIPTION_ID"; exit 1; }
	@test -n "$(CVM_BOOTSTRAP_RESOURCE_GROUP_NAME)" || { echo "CVM final build requires CVM_BOOTSTRAP_RESOURCE_GROUP_NAME"; exit 1; }
	@test -n "$(CVM_BOOTSTRAP_SIG_GALLERY_NAME)" || { echo "CVM final build requires CVM_BOOTSTRAP_SIG_GALLERY_NAME"; exit 1; }
	@test -n "$(CVM_BOOTSTRAP_SIG_IMAGE_NAME)" || { echo "CVM final build requires CVM_BOOTSTRAP_SIG_IMAGE_NAME"; exit 1; }
	@test -n "$(CVM_BOOTSTRAP_SIG_IMAGE_VERSION)" || { echo "CVM final build requires CVM_BOOTSTRAP_SIG_IMAGE_VERSION"; exit 1; }

init-packer-cvm-bootstrap:
	@CVM_BUILD_STAGE=bootstrap ./vhdbuilder/packer/produce-packer-settings.sh

build-packer-cvm-bootstrap:
	@echo "Using packer template file vhd-image-builder-cvm-bootstrap.json"
	@packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-cvm-bootstrap.json

run-packer-cvm-bootstrap: validate-cvm-two-stage az-login
	@packer init ./vhdbuilder/packer/packer-plugin.pkr.hcl && packer version && ($(MAKE) -f packer.mk init-packer-cvm-bootstrap | tee $(CVM_BOOTSTRAP_PACKER_OUTPUT)) && ($(MAKE) -f packer.mk build-packer-cvm-bootstrap | tee -a $(CVM_BOOTSTRAP_PACKER_OUTPUT))

init-packer-cvm-final:
	@CVM_BUILD_STAGE=final ./vhdbuilder/packer/produce-packer-settings.sh

build-packer-cvm-final: setup-golang generate-prefetch-scripts build-image-fetcher build-aks-node-controller build-lister-binary
	@echo "Using packer template file $(CVM_FINAL_TEMPLATE)"
	@packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json $(CVM_FINAL_TEMPLATE)

run-packer-cvm-final: validate-cvm-final az-login
	@packer init ./vhdbuilder/packer/packer-plugin.pkr.hcl && packer version && ($(MAKE) -f packer.mk init-packer-cvm-final | tee packer-output) && ($(MAKE) -f packer.mk build-packer-cvm-final | tee -a packer-output)

generate-publishing-info: az-login
	@./vhdbuilder/packer/generate-vhd-publishing-info.sh

convert-sig-to-classic-storage-account-blob: az-login
	@./vhdbuilder/packer/convert-sig-to-classic-storage-account-blob.sh

scanning-vhd: az-login
	@./vhdbuilder/packer/vhd-scanning.sh

test-scan-and-cleanup: az-login
	@./vhdbuilder/packer/test-scan-and-cleanup.sh

replicate-captured-sig-image-version: az-login
	@./vhdbuilder/packer/replicate-captured-sig-image-version.sh

evaluate-build-performance: az-login
	@./vhdbuilder/packer/buildperformance/evaluate-build-performance.sh

evaluate-grid-compatibility: az-login
	@./vhdbuilder/packer/gridcompatibility/evaluate-grid-compatibility.sh

prefetch: az-login
	@./vhdbuilder/prefetch/scripts/optimize.sh

cleanup-prefetch: az-login
	@./vhdbuilder/prefetch/scripts/cleanup.sh

generate-prefetch-scripts:
	@echo "${MODE}: Generating prefetch scripts"
	@bash -c "pushd vhdbuilder/prefetch; go run cmd/main.go --components-path=../../parts/common/components.json --postfix-path=../../parts/linux/cloud-init/artifacts/cse_preload.sh --output-path=../packer/prefetch.sh || exit 1; popd"

setup-golang:
	@echo "Setting up Go environment"
	@bash ./hack/setup_golang.sh

build-aks-node-controller:
	@echo "Building aks-node-controller binaries"
	@bash -c 'set -euo pipefail; \
	cd aks-node-controller; \
	go test ./...; \
	ANC_VERSION="$${IMAGE_VERSION:-$$(date +%Y%m.%d.0)}"; \
	ANC_LDFLAGS="-X main.Version=$${ANC_VERSION}"; \
	echo "Stamping ANC version: $${ANC_VERSION}"; \
	GOEXPERIMENT=ms_nocgo_opensslcrypto CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$${ANC_LDFLAGS}" -o bin/aks-node-controller-linux-amd64; \
	GOEXPERIMENT=ms_nocgo_opensslcrypto CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$${ANC_LDFLAGS}" -o bin/aks-node-controller-linux-arm64'

build-image-fetcher:
	@echo "Building image-fetcher binaries"
	@bash -c "pushd image-fetcher && \
	GOEXPERIMENT=ms_nocgo_opensslcrypto CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/image-fetcher-linux-amd64 && \
	GOEXPERIMENT=ms_nocgo_opensslcrypto CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/image-fetcher-linux-arm64 && \
	popd"

build-lister-binary:
	@echo "Building lister binary for $(GOARCH)"
	@bash -c "pushd vhdbuilder/lister && GOEXPERIMENT=ms_nocgo_opensslcrypto CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o bin/lister main.go && popd"

generate-acl-customdata: vhdbuilder/packer/acl-customdata.json
vhdbuilder/packer/acl-customdata.json: vhdbuilder/packer/acl-customdata.yaml | hack/tools/bin/butane
	@hack/tools/bin/butane --strict $< -o $@

publish-imagecustomizer:
	@echo "Publishing VHD generated by imagecustomizer"
	@./vhdbuilder/packer/imagecustomizer/scripts/publish-imagecustomizer-image.sh

hack/tools/bin/butane:
	@echo "Building butane for $(GOHOSTARCH)"
	@bash -c "pushd hack/tools && GOARCH=$(GOHOSTARCH) make $(shell pwd)/$@"
