#!/usr/bin/env bash
set -euxo pipefail

mkdir -p /mnt/sdb1
mount /dev/sdb1 /mnt/sdb1
cd /mnt/sdb1/vhdbuilder/packer

ls

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