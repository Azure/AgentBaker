SHELL=/bin/bash -o pipefail

build-packer:
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
	@echo "${MODE}: Convert os disk snapshot to SIG"
	@./vhdbuilder/packer/convert-osdisk-snapshot-to-sig.sh
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
	@az login --identity
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

windows-vhd-publishing-info: az-login
	@./vhdbuilder/packer/generate-windows-vhd-publishing-info.sh

test-building-vhd: az-login
	@./vhdbuilder/packer/test/run-test.sh
