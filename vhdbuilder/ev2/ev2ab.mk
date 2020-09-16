export EV2AB_VERSION       ?= 9.7.0
# Shared makefile for ev2ab
# version 1.0
# Source: https://dev.azure.com/msazure/CloudNativeCompute/_git/aks-deployment?path=%2Fmakefiles%2Fev2ab.mk
# $(BUILD_DIR)/tool/       | ev2ab tool 
# $(BUILD_DIR)/source/     | ev2 artifact source
# $(OUTPUT_DIR)/           | ev2 artifact

PROJECT_ROOT        ?= $(abspath $(dir $(lastword $(MAKEFILE_LIST)))..)
BUILD_DIR           ?= $(PROJECT_ROOT)/build/ev2
OUTPUT_DIR          ?= $(BUILD_DIR)/output
EV2AB               := $(BUILD_DIR)/tools/ev2ab/ev2ab.sh
EV2AB_SOURCE        := $(BUILD_DIR)/source
EV2_BUILDVERSION    ?= $(BUILD_BUILDNUMBER)

# prepare ev2ab tool
$(EV2AB): | ensure-azure-cli-extension-ado
ifeq ($(EV2AB_VERSION),)
	$(error EV2AB_VERSION must be set)
endif
	@echo "Download ev2ab version $(EV2AB_VERSION)"
	@mkdir -p "$(@D)"
	@az artifacts universal download \
		--organization "https://dev.azure.com/msazure/" \
		--project CloudNativeCompute \
		--scope project \
		--feed aks-deployment \
		--name ev2ab-aks \
		--version "${EV2AB_VERSION}" \
		--path "$(@D)"

.PHONY: ensure-azure-cli-extension-ado
ensure-azure-cli-extension-ado: | ensure-azure-cli
	@echo "Ensure azure-cli azure-devops extension"
	@if ! az extension show -n azure-devops > /dev/null; then \
		az extension add --name azure-devops > /dev/null; \
	fi

.PHONY: ensure-azure-cli
ensure-azure-cli:
	@echo "Ensure azure-cli"
	@if ! command -v az > /dev/null; then \
		curl -sL "https://aka.ms/InstallAzureCLIDeb" | bash > /dev/null; \
	fi

EV2AB_DIRS := $(patsubst %/ev2ab.yaml,%,$(wildcard */ev2ab.yaml))

$(OUTPUT_DIR)/ev2ab_manifest.txt: $(EV2AB) $(addprefix $(EV2AB_SOURCE)/,$(EV2AB_DIRS))
ifeq ($(EV2_BUILDVERSION),)
	$(error EV2_BUILDVERSION must be set)
endif
	$(info EV2_BUILDVERSION=$(EV2_BUILDVERSION))
	export EV2_BUILDVERSION && bash $(EV2AB) $(EV2AB_SOURCE) $(OUTPUT_DIR)
	@echo $(EV2AB_DIRS) > $@

.PHONY: prepare
prepare: $(EV2AB)

.PHONY: all
all: $(OUTPUT_DIR)/ev2ab_manifest.txt
