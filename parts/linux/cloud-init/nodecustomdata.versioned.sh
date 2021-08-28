#!/usr/bin/env bash
set -eux

if systemctl is-active containerd; then 
  echo "containerd not running"
  exit 1
fi

echo "regenerating payload"
ctr run --mount type=bind,src=/opt,dst=/opt --rm {{ GetParameter "bakerRegisry" }}/baker:{{ GetParameter "bakerVersion" }} baker /usr/local/bin/baker {{ToPrettyJson .}}
echo "removing semaphores"
rm /var/lib/cloud/instance/sem/config_cc_write_files
rm /var/lib/cloud/instance/sem/config_runcmd
rm /var/lib/cloud/instance/sem/config_scripts_user
echo "rerunning cc_write_files"
cloud-init --file /opt/azure/containers/cse_payload.txt single -n cc_write_files
echo "rerunning runcmd"
cloud-init --file /opt/azure/containers/cse_payload.txt single -n runcmd
echo "rerunning cc_write_files"
cloud-init --file /opt/azure/containers/cse_payload.txt single -n scripts_user
"executing regenerated CSE file"
bash /opt/azure/containers/cse_regen.sh
