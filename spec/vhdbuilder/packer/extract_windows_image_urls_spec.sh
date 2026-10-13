#!/bin/bash

Describe 'extract_windows_image_urls function'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  setup_environment() {
    artifact_path="./spec/parts/linux/cloud-init/artifacts/sample_payload.json"
    WINDOWS_SKU=""
    WINDOWS_BASE_IMAGE_URL=""
    windows_nanoserver_image_url=""
    windows_servercore_image_url=""
  }

  extract_2025_gen2_and_trusted_launch_urls() {
    WINDOWS_SKU="2025-gen2"
    extract_windows_image_urls >/dev/null || return 1
    gen2_base_image_url="$WINDOWS_BASE_IMAGE_URL"
    gen2_nanoserver_image_url="$windows_nanoserver_image_url"
    gen2_servercore_image_url="$windows_servercore_image_url"

    WINDOWS_SKU="2025-gen2-tl"
    extract_windows_image_urls >/dev/null || return 1

    [ "$WINDOWS_BASE_IMAGE_URL" = "$gen2_base_image_url" ] &&
      [ "$windows_nanoserver_image_url" = "$gen2_nanoserver_image_url" ] &&
      [ "$windows_servercore_image_url" = "$gen2_servercore_image_url" ]
  }

  BeforeEach 'setup_environment'

  It 'extracts a Gen2 base image URL'
    WINDOWS_SKU="2022-containerd-gen2"

    When call extract_windows_image_urls
    The status should be success
    The output should include "Reading image URLs from ./spec/parts/linux/cloud-init/artifacts/sample_payload.json"
    The variable WINDOWS_BASE_IMAGE_URL should include "/ws2022/GEN2/2022-datacenter-core-smalldisk-g2-sim.vhd"
    The variable windows_nanoserver_image_url should include "/ws2022/CONTAINERS/nanoserver.tar"
    The variable windows_servercore_image_url should include "/ws2022/CONTAINERS/servercore.tar"
  End

  It 'extracts both Windows 2025 and 2022 container image URLs'
    WINDOWS_SKU="2025"

    When call extract_windows_image_urls
    The status should be success
    The output should include "Reading image URLs from ./spec/parts/linux/cloud-init/artifacts/sample_payload.json"
    The variable WINDOWS_BASE_IMAGE_URL should include "/ws2025/2025-datacenter-core-smalldisk-sim.vhd"
    The variable windows_nanoserver_image_url should include "/ws2025/CONTAINERS/nanoserver.tar,https://"
    The variable windows_nanoserver_image_url should include "/ws2022/CONTAINERS/nanoserver.tar"
    The variable windows_servercore_image_url should include "/ws2025/CONTAINERS/servercore.tar,https://"
    The variable windows_servercore_image_url should include "/ws2022/CONTAINERS/servercore.tar"
  End

  It 'uses the same payload arguments for Windows 2025 Gen2 and Trusted Launch'
    When call extract_2025_gen2_and_trusted_launch_urls
    The status should be success
    The variable WINDOWS_BASE_IMAGE_URL should include "/ws2025/GEN2/2025-datacenter-core-smalldisk-g2-sim.vhd"
    The variable windows_nanoserver_image_url should include "/ws2025/CONTAINERS/nanoserver.tar,https://"
    The variable windows_nanoserver_image_url should include "/ws2022/CONTAINERS/nanoserver.tar"
    The variable windows_servercore_image_url should include "/ws2025/CONTAINERS/servercore.tar,https://"
    The variable windows_servercore_image_url should include "/ws2022/CONTAINERS/servercore.tar"
  End

  It 'rejects an unsupported SKU'
    WINDOWS_SKU="unsupported"

    When call extract_windows_image_urls
    The status should be failure
    The output should include "Unsupported WINDOWS_SKU: unsupported"
  End
End
