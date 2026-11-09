#!/bin/bash
set -euo pipefail

base_template=${ACL_PACKER_TEMPLATE:-vhdbuilder/packer/vhd-image-builder-acl.json}

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
# Packer picks its parser from the file extension, so keep the .json suffix.
kata_template="${workdir}/vhd-image-builder-acl-kata.json"

# The ACL Kata base image ships the Kata host packages. Upload the Kata guest image (UVM) and its
# version before the build scripts run, and check the version against the installed host package
# once install-dependencies.sh is done, as vhd-image-builder-mariner.json does for Azure Linux Kata.
jq '
  def provisioner_index($script):
    [.provisioners[] | (.inline // "") | tostring | contains("/bin/bash -eux /home/packer/" + $script)] | index(true)
    // error("no provisioner runs " + $script + " in the base template");
  provisioner_index("pre-install-dependencies.sh") as $pre |
  provisioner_index("install-dependencies.sh") as $install |
  .variables += {
    "kata_uvm_version": "{{env `KATA_UVM_VERSION`}}",
    "system_collection_uri": "{{env `SYSTEM_COLLECTIONURI`}}",
    "system_team_project_id": "{{env `SYSTEM_TEAMPROJECTID`}}"
  } |
  .provisioners = .provisioners[:$pre] + [
    {
      "type": "file",
      "source": "kata-containers.img",
      "destination": "/home/packer/kata-containers.img"
    },
    {
      "type": "file",
      "content": "{{user `kata_uvm_version`}}\n",
      "destination": "/home/packer/kata-containers-version.txt"
    }
  ] + .provisioners[$pre:$install + 1] + [
    {
      "type": "shell",
      "script": "vhdbuilder/packer/validate-kata-uvm-version.sh",
      "environment_vars": [
        "FEATURE_FLAGS={{user `feature_flags`}}",
        "OS_SKU=AzureContainerLinux",
        "SYSTEM_COLLECTIONURI={{user `system_collection_uri`}}",
        "SYSTEM_TEAMPROJECTID={{user `system_team_project_id`}}"
      ]
    }
  ] + .provisioners[$install + 1:]
' "$base_template" > "$kata_template"

echo "Using ACL Kata image settings derived from $base_template"
packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$kata_template"
