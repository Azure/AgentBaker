#!/bin/bash

Describe 'vhd-image-builder-cvm-2604.json'
  standard_template='vhdbuilder/packer/vhd-image-builder-cvm.json'
  cvm_2604_template='vhdbuilder/packer/vhd-image-builder-cvm-2604.json'

  It 'uses the bootstrap Shared Image Gallery image as its source'
    The value "$(jq -r '.builders[0] | has("image_publisher")' "${cvm_2604_template}")" should eq "false"
    The value "$(jq -r '.builders[0] | has("image_offer")' "${cvm_2604_template}")" should eq "false"
    The value "$(jq -r '.builders[0] | has("image_sku")' "${cvm_2604_template}")" should eq "false"
    The value "$(jq -r '.builders[0] | has("image_version")' "${cvm_2604_template}")" should eq "false"
    The value "$(jq -r '.builders[0].shared_image_gallery.subscription' "${cvm_2604_template}")" should eq '{{user `cvm_bootstrap_subscription_id`}}'
    The value "$(jq -r '.builders[0].shared_image_gallery.resource_group' "${cvm_2604_template}")" should eq '{{user `cvm_bootstrap_resource_group_name`}}'
    The value "$(jq -r '.builders[0].shared_image_gallery.gallery_name' "${cvm_2604_template}")" should eq '{{user `cvm_bootstrap_sig_gallery_name`}}'
    The value "$(jq -r '.builders[0].shared_image_gallery.image_name' "${cvm_2604_template}")" should eq '{{user `cvm_bootstrap_sig_image_name`}}'
    The value "$(jq -r '.builders[0].shared_image_gallery.image_version' "${cvm_2604_template}")" should eq '{{user `cvm_bootstrap_sig_image_version`}}'
  End

  It 'retains the standard CVM security and destination configuration'
    The value "$(jq -r '.builders[0].security_type' "${cvm_2604_template}")" should eq "ConfidentialVM"
    The value "$(jq -r '.builders[0].secure_boot_enabled' "${cvm_2604_template}")" should eq "true"
    The value "$(jq -r '.builders[0].vtpm_enabled' "${cvm_2604_template}")" should eq "true"
    The value "$(jq -r '.builders[0].shared_image_gallery_destination.specialized' "${cvm_2604_template}")" should eq "true"
  End

  It 'keeps the standard CVM provisioners and cleanup behavior'
    The value "$(diff <(jq -S '.provisioners' "${standard_template}") <(jq -S '.provisioners' "${cvm_2604_template}"))" should eq ""
    The value "$(diff <(jq -S '."error-cleanup-provisioner"' "${standard_template}") <(jq -S '."error-cleanup-provisioner"' "${cvm_2604_template}"))" should eq ""
  End
End
