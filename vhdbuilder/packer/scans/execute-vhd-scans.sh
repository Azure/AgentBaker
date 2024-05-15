#!/usr/bin/env bash
set -euxo pipefail

mkdir -p /mnt/sda1
mount /dev/sda1 /mnt/sda1

/home/packer/storage-scan.sh
/home/packer/trivy-scan.sh

umount /mnt/sda1
rmdir /mnt/sda1

#
#echo "Run Trivy and Storage Scans"
#cd $CDIR
#cd ../
# var/lib/waagent/run-command/download/1
#az vm run-command invoke --resource-group $RESOURCE_GROUP_NAME --name $VM_NAME --command-id RunShellScript --scripts "trivy-scan.sh"
#az vm run-command invoke --resource-group $RESOURCE_GROUP_NAME --name $VM_NAME --command-id RunShellScript --scripts "storage-scan.sh"

#IP="10.0.0.4"
#TRIVY_REPORT_JSON_PATH=/opt/azure/containers/trivy-report.json
#TRIVY_REPORT_TABLE_PATH=/opt/azure/containers/trivy-images-table.txt
#STORAGE_REPORT_PATH=/opt/azure/containers/storage-report.txt
# scp -r azureuser@myserver.eastus.cloudapp.com:/home/azureuser/logs/. /tmp/
#scp -r $TEST_VM_ADMIN_USERNAME@$IP:$TRIVY_REPORT_JSON_PATH .
#scp -r $TEST_VM_ADMIN_USERNAME@$IP:$TRIVY_REPORT_TABLE_PATH .
#scp -r $TEST_VM_ADMIN_USERNAME@$IP:$STORAGE_REPORT_PATH .

#az vm show --resource-group $RESOURCE_GROUP_NAME --name $VM_NAME --query fqdns --output tsv

# az vm create --resource-group yourResourceGroup --name yourVMName --image yourImage --size yourVMSize --os-disk-size-gb 50 --admin-username yourAdminUsername --admin-password yourAdminPassword
# az vm create --resource-group alison-dpd-rg --name vhd-test-vm --image /subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804/versions/1.1715620673.9522 --os-disk-size-gb 50 --admin-username azureuser --admin-password TestVM@1715622512 --public-ip-address ''