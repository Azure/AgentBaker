SHELL=/bin/bash -o pipefail

build-packer:
ifeq (${OS_SKU},Ubuntu)
ifeq (${ARCHITECTURE},ARM64)
ifeq (${HYPERV_GENERATION},V2)
	@echo "${MODE}: Building with Hyper-v generation 2 ARM64 VM"
	@echo "Using packer template file vhd-image-builder-arm64-gen2.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
	@echo "${MODE}: Convert os disk snapshot to SIG"
	@./vhdbuilder/packer/convert-osdisk-snapshot-to-sig.sh
endif
else
ifeq (${HYPERV_GENERATION},V2)
	@echo "${MODE}: Building with Hyper-v generation 2 VM"
else
	@echo "${MODE}: Building with Hyper-v generation 1 VM"
endif
	@echo "Using packer template file: vhd-image-builder-base.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-base.json
endif
else ifeq (${OS_SKU},CBLMariner)
ifeq (${OS_VERSION},V1)
ifeq (${HYPERV_GENERATION},V2)
	@echo "${MODE}: Building with Hyper-v generation 2 VM"
	@echo "Using packer template file vhd-image-builder-mariner-gen2.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-gen2.json
else
	@echo "${MODE}: Building with Hyper-v generation 1 VM"
	@echo "Using packer template file vhd-image-builder-mariner.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner.json
endif
else ifeq (${OS_VERSION},V2)
ifeq (${ARCHITECTURE}, ARM64)
ifeq (${HYPERV_GENERATION},V2)
	@echo "${MODE}: Building with Hyper-v generation 2 ARM64 VM"
	@echo "Using packer template file vhd-image-builder-mariner2-arm64.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner2-arm64.json
	@echo "${MODE}: Convert os disk snapshot to SIG"
	@./vhdbuilder/packer/convert-osdisk-snapshot-to-sig.sh
endif
else
ifeq (${HYPERV_GENERATION},V2)
	@echo "${MODE}: Building with Hyper-v generation 2 VM"
	@echo "Using packer template file vhd-image-builder-mariner2-gen2.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner2-gen2.json
else
	@echo "${MODE}: Building with Hyper-v generation 1 VM"
	@echo "Using packer template file vhd-image-builder-mariner2-gen2.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner2-gen2.json
endif
endif
else ifeq (${OS_VERSION},V2kata)
ifeq (${HYPERV_GENERATION},V2)
	@echo "${MODE}: Building with Hyper-v generation 2 VM for kata"
	@echo "Using packer template file vhd-image-builder-mariner2-gen2-kata.json"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner2-gen2-kata.json
endif
else
	$(error OS_VERSION was invalid ${OS_VERSION})
endif
else
	$(error OS_SKU was invalid ${OS_SKU})
endif

build-packer-windows:
ifeq (${MODE},windowsVhdMode)
ifeq (${SIG_FOR_PRODUCTION},True)
ifeq (${HYPERV_GENRATION},V1)
	@echo "${MODE}: Building with Hyper-v generation 1 VM and save to Classic Storage Account"
else
	@echo "${MODE}: Building with Hyper-v generation 2 VM and save to Classic Storage Account"
endif
else
ifeq (${HYPERV_GENRATION},V1)
	@echo "${MODE}: Building with Hyper-v generation 1 VM and save to Shared Image Gallery"
else
	@echo "${MODE}: Building with Hyper-v generation 2 VM and save to Shared Image Gallery"
endif
endif
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows-vhd-builder-sig.json
endif

az-login:
ifeq (${OS_TYPE},Windows)
	@echo "Logging into Azure with service principal..."
	@az login --service-principal -u ${CLIENT_ID} -p ${CLIENT_SECRET} --tenant ${TENANT_ID}
else
	@echo "Logging into Azure with agent VM MSI..."
	@az login --identity
endif
	@az account set -s ${SUBSCRIPTION_ID}

init-packer:
	@./vhdbuilder/packer/init-variables.sh

run-packer: az-login
	@packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer | tee -a packer-output)

run-packer-windows: az-login
	@packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer-windows | tee -a packer-output)

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
