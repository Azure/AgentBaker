build-packer:
ifeq (${HYPERV_GENERATION},V2)
	@echo "Building with Hyper-v generation 2 VM"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder-gen2.json
else
	@echo "Building with Hyper-v generation 1 VM"
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/vhd-image-builder.json
endif

build-packer-windows:
	@packer build -var-file=vhdbuilder/packer/settings.json vhdbuilder/packer/windows-vhd-builder.json

init-packer:
	@./vhdbuilder/packer/init-variables.sh

az-login:
	az login --service-principal -u ${CLIENT_ID} -p ${CLIENT_SECRET} --tenant ${TENANT_ID}

run-packer: az-login
	@packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer | tee -a packer-output)

run-packer-windows: az-login
	@packer version && ($(MAKE) -f packer.mk init-packer | tee packer-output) && ($(MAKE) -f packer.mk build-packer-windows | tee -a packer-output)

az-copy: az-login
	azcopy-preview copy "${OS_DISK_SAS}" "${CLASSIC_BLOB}${CLASSIC_SAS_TOKEN}" --recursive=true

delete-sa: az-login
	az storage account delete -n ${SA_NAME} -g ${AZURE_RESOURCE_GROUP_NAME} --yes

generate-sas: az-login
	@./vhdbuilder/packer/generate-vhd-publishing-info.sh

generate-sig-info: az-login
	@./vhdbuilder/packer/generate-vhd-publishing-info-gen2.sh