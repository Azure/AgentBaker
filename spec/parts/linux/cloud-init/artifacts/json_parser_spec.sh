#!/bin/bash

# File: spec/parts/linux/cloud-init/artifacts/json_parser_spec.sh
Describe 'json_parser.sh'
    Include "./spec/parts/linux/cloud-init/artifacts/json_parser.sh"
    test -f ./spec/parts/linux/cloud-init/artifacts/sample_payload.json && echo "File exists"
    chmod 644 ./spec/parts/linux/cloud-init/artifacts/sample_payload.json
    json_file="./spec/parts/linux/cloud-init/artifacts/sample_payload.json"
    BeforeAll 'json_file="./spec/parts/linux/cloud-init/artifacts/sample_payload.json"'

    It 'confirms the JSON file exists'
        When run test -f "$json_file"
        The status should be success
    End

    Describe 'get_image_url_using_name'
        It 'extracts the correct image URL for WINDOWS_2019_BASE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2019_BASE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2019/2019-datacenter-core-smalldisk-sim.vhd"
        End
        It 'extracts the correct image URL for WINDOWS_2019_CORE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2019_CORE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2019/CONTAINERS/servercore.tar"
        End
        It 'extracts the correct image URL for WINDOWS_2019_NANO_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2019_NANO_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2019/CONTAINERS/nanoserver.tar"
        End
        It 'extracts the correct image URL for WINDOWS_2022_BASE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2022_BASE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2022/2022-datacenter-core-smalldisk-sim.vhd"
        End
        It 'extracts the correct image URL for WINDOWS_2022_CORE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2022_CORE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2022/CONTAINERS/servercore.tar"
        End
        It 'extracts the correct image URL for WINDOWS_2022_NANO_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2022_NANO_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2022/CONTAINERS/nanoserver.tar"
        End
        It 'extracts the correct image URL for WINDOWS_2022_GEN2_BASE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2022_GEN2_BASE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2022/GEN2/2022-datacenter-core-smalldisk-g2-sim.vhd"
        End
        It 'extracts the correct image URL for WINDOWS_2025_BASE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2025_BASE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2025/2025-datacenter-core-smalldisk-sim.vhd"
        End
        It 'extracts the correct image URL for WINDOWS_2025_CORE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2025_CORE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2025/CONTAINERS/servercore.tar"
        End
        It 'extracts the correct image URL for WINDOWS_2025_NANO_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2025_NANO_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2025/CONTAINERS/nanoserver.tar"
        End
        It 'extracts the correct image URL for WINDOWS_2025_GEN2_BASE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_2025_GEN2_BASE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws2025/GEN2/2025-datacenter-core-smalldisk-g2-sim.vhd"
        End
        It 'extracts the correct image for WINDOWS_23H2_BASE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_23H2_BASE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws23H2/23h2-datacenter-core-sim.vhd"
        End
        It 'extracts the correct image for WINDOWS_23H2_GEN2_BASE_IMAGE_URL'
            When call get_image_url_using_name "$json_file" "WINDOWS_23H2_GEN2_BASE_IMAGE_URL"
            The output should equal "https://wcctagentbakerstorage.blob.core.windows.net/simship/2025/08B/ws23H2/GEN2/23h2-datacenter-core-g2-sim.vhd"
        End
    End
End