#!/bin/bash
set -eux
RESOURCE_GROUP_NAME="aksvhdtestbuildrg"
VM_NAME="alison-vhd-test-vm"
TEST_VM_ADMIN_USERNAME="azureuser"
TEST_VM_ADMIN_PASSWORD="TestVM@1715622512"
IP="10.0.0.4"
VHD_IMAGE="/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804/versions/1.1715620673.9522"
BASTION_NAME="vhdtest-bastion"
FQDN="alisontestfqdn"

#function cleanup() {
#  az vm delete --name $VM_NAME --resource-group $RESOURCE_GROUP_NAME --yes
#}
#trap cleanup EXIT


# create the VM
#az vm create --resource-group $RESOURCE_GROUP_NAME \
#    --name $VM_NAME \
#    --image $VHD_IMAGE \
#    --admin-username $TEST_VM_ADMIN_USERNAME \
#    --admin-password $TEST_VM_ADMIN_PASSWORD \
#    --os-disk-size-gb 50 \
#    --public-ip-address-dns-name $FQDN \
#    --location eastus


# create the packer directory
#az vm run-command invoke --resource-group $RESOURCE_GROUP_NAME --name $VM_NAME --command-id RunShellScript --scripts "sudo mkdir /home/packer"
