SHELL=/bin/bash -o pipefail

GOARCH=amd64
ifeq (${ARCHITECTURE},ARM64)
	GOARCH=arm64
endif
GOHOSTARCH = $(shell go env GOHOSTARCH)

build-packer: generate-prefetch-scripts build-aks-node-controller build-lister-binary
ifeq (${ARCHITECTURE},ARM64)
	@echo "${MODE}: Building with Hyper-v generation 2 ARM64 VM"
ifeq (${OS_SKU},Ubuntu)
	@echo "Using packer template file vhd-image-builder-arm64-gen2.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
else ifeq (${OS_SKU},CBLMariner)
	@echo "Using packer template file vhd-image-builder-mariner-arm64.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
else ifeq (${OS_SKU},AzureLinux)
	@echo "Using packer template file vhd-image-builder-mariner-arm64.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
else ifeq (${OS_SKU},Flatcar)
	@echo "Using packer template file vhd-image-builder-flatcar-arm64.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-flatcar-arm64.json
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
else ifeq (${OS_SKU},Flatcar)
	@echo "Using packer template file vhd-image-builder-flatcar.json"
	@packer build -timestamp-ui  -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-flatcar.json
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

build-imagecustomizer: generate-prefetch-scripts build-aks-node-controller build-lister-binary
	@./vhdbuilder/packer/imagecustomizer/scripts/build-imagecustomizer-image.sh

az-login:
ifeq (${MODE},windowsVhdMode)
ifeq ($(origin MANAGED_IDENTITY_ID), undefined)
	@echo "Logging in with Hosted Pool's Default Managed Identity"
	@az login --identity
else
	@echo "Logging in with Hosted Pool's Managed Identity: ${MANAGED_IDENTITY_ID}"
	@az login --identity --client-id ${MANAGED_IDENTITY_ID}
endif
else
	@echo "Logging into Azure with identity: ${AZURE_MSI_RESOURCE_STRING}..."
	@az login --identity --resource-id ${AZURE_MSI_RESOURCE_STRING}
endif
	@echo "Using the subscription ${SUBSCRIPTION_ID}"
	@az account set -s ${SUBSCRIPTION_ID}

init-packer:
	@./vhdbuilder/packer/produce-packer-settings.sh ${AZCLI_VERSION_OVERRIDE}

run-packer: az-login
	@packer init ./vhdbuilder/packer/packer-plugin.pkr.hcl && packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer | tee -a packer-output)

run-imagecustomizer: az-login
	@($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-imagecustomizer | tee -a packer-output)

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

generate-prefetch-scripts:
#ifeq (${MODE},linuxVhdMode)
	@echo "${MODE}: Generating prefetch scripts"
	@bash -c "pushd vhdbuilder/prefetch; go run cmd/main.go --components-path=../../parts/common/components.json --output-path=../packer/prefetch.sh || exit 1; popd"
#endif

build-aks-node-controller:
	@echo "Building aks-node-controller binaries"
	@bash -c "pushd aks-node-controller && \
	go test ./... && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/aks-node-controller-linux-amd64 && \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/aks-node-controller-linux-arm64 && \
	popd"

build-lister-binary:
	@echo "Building lister binary for $(GOARCH)"
	@bash -c "pushd vhdbuilder/lister && CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o bin/lister main.go && popd"

generate-flatcar-customdata: vhdbuilder/packer/flatcar-customdata.json
vhdbuilder/packer/flatcar-customdata.json: vhdbuilder/packer/flatcar-customdata.yaml | hack/tools/bin/butane
	@hack/tools/bin/butane --strict $< -o $@

publish-imagecustomizer:
	@echo "Publishing VHD generated by imagecustomizer"
	@./vhdbuilder/packer/imagecustomizer/scripts/publish-imagecustomizer-image.sh

hack/tools/bin/butane:
	@echo "Building butane for $(GOHOSTARCH)"
	@bash -c "pushd hack/tools && GOARCH=$(GOHOSTARCH) make $(shell pwd)/$@"
