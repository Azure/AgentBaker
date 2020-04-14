build-packer:
ifeq (${MODE},mode1)
	@echo "${MODE}: Building with Hyper-v generation 2 VM"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-gen2.json
else ifeq (${MODE},mode2)
	@echo "${MODE}: Building with Hyper-v generation 1 VM and save to Shared Image Gallery"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-sig.json
else
	@echo "${MODE}: Building with Hyper-v generation 1 VM and save to Classic Storage Account"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder.json
endif

build-packer-windows:
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows-vhd-builder.json

init-packer:
	@./vhdbuilder/packer/init-variables.sh

az-login:
	az login --service-principal -u ${CLIENT_ID} -p ${CLIENT_SECRET} --tenant ${TENANT_ID}
	az account set -s ${SUBSCRIPTION_ID}

run-packer: az-login
	@packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer | tee -a packer-output)

run-packer-windows: az-login
	@packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer-windows | tee -a packer-output)

az-copy: az-login
	azcopy-preview copy "${OS_DISK_SAS}" "${CLASSIC_BLOB}${CLASSIC_SAS_TOKEN}" --recursive=true

delete-sa: az-login
	az storage account delete -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --yes

delete-mi: az-login
	az image delete -n ${IMAGE_NAME} -g ${AZURE_RESOURCE_GROUP_NAME}

generate-sas: az-login
	@./vhdbuilder/packer/generate-vhd-publishing-info.sh

create-managed-disk-from-sig: az-login
	@./vhdbuilder/packer/create-managed-disk-from-sig-gen2.sh