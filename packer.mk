SHELL=/bin/bash -o pipefail

GOARCH=amd64
ifeq (${ARCHITECTURE},ARM64)
	GOARCH=arm64
endif

build-packer: generate-prefetch-scripts build-nbcparser-all build-lister-binary
ifeq (${ARCHITECTURE},ARM64)
	@echo "${MODE}: Building with Hyper-v generation 2 ARM64 VM"
ifeq (${OS_SKU},Ubuntu)
	@echo "Using packer template file vhd-image-builder-arm64-gen2.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
else ifeq (${OS_SKU},CBLMariner)
	@echo "Using packer template file vhd-image-builder-mariner-arm64.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
else ifeq (${OS_SKU},AzureLinux)
	@echo "Using packer template file vhd-image-builder-mariner-arm64.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
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
	@echo "Using packer template file: vhd-image-builder-base.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-base.json
else ifeq (${OS_SKU},CBLMariner)
	@echo "Using packer template file vhd-image-builder-mariner.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner.json
else ifeq (${OS_SKU},AzureLinux)
	@echo "Using packer template file vhd-image-builder-mariner.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner.json
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
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows-vhd-builder-sig.json
endif

az-login:
	@echo "Logging into Azure with agent VM MSI..."
ifeq ($(origin MANAGED_IDENTITY_ID), undefined)
	@echo "Logging in with Hosted Pool's Default Managed Identity"
	@az login --identity
else
	@echo "Logging in with Hosted Pool's Managed Identity: ${MANAGED_IDENTITY_ID}"
	@az login --identity --username ${MANAGED_IDENTITY_ID}
endif
	@echo "Using the subscription ${SUBSCRIPTION_ID}"
	@az account set -s ${SUBSCRIPTION_ID}

init-packer:
	@./vhdbuilder/packer/init-variables.sh

run-packer: az-login
	@packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer | tee -a packer-output)

run-packer-windows: az-login
	@packer init ./vhdbuilder/packer/packer-plugin.pkr.hcl && packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer-windows | tee -a packer-output)

cleanup: az-login
	@./vhdbuilder/packer/cleanup.sh

backfill-cleanup: az-login
	@chmod +x ./vhdbuilder/packer/backfill-cleanup.sh
	@./vhdbuilder/packer/backfill-cleanup.sh

generate-sas: az-login
	@./vhdbuilder/packer/generate-vhd-publishing-info.sh

convert-sig-to-classic-storage-account-blob: az-login
	@./vhdbuilder/packer/convert-sig-to-classic-storage-account-blob.sh

test-building-vhd: az-login
	@./vhdbuilder/packer/test/run-test.sh

scanning-vhd: az-login
	@./vhdbuilder/packer/vhd-scanning.sh

test-scan-and-cleanup: az-login
	@./vhdbuilder/packer/test-scan-and-cleanup.sh

evaluate-build-performance: az-login
	@./vhdbuilder/packer/build-performance/evaluate-build-performance.sh

generate-prefetch-scripts:
ifeq (${MODE},linuxVhdMode)
	@echo "${MODE}: Generating prefetch scripts"
	@bash -c "pushd vhdbuilder/prefetch; go run main.go --components-path=../../parts/linux/cloud-init/artifacts/components.json --output-path=../packer/prefetch.sh || exit 1; popd"
endif

build-nbcparser-all:
	@$(MAKE) -f packer.mk build-nbcparser-binary ARCH=amd64
	@$(MAKE) -f packer.mk build-nbcparser-binary ARCH=arm64

build-nbcparser-binary:
	@echo "Building nbcparser binary"
	@bash -c "pushd nbcparser && CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -o bin/nbcparser-$(ARCH) main.go && popd"

build-lister-binary:
	@echo "Building lister binary for $(GOARCH)"
	@bash -c "pushd vhdbuilder/lister && CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o bin/lister main.go && popd"