#!/bin/bash
set -euxo pipefail

az login --identity
build_id="76289801"

something_to_do() {
    build_id="76289801"
    echo "Build ID for the release is $build_id"
    pipeline_id="322622" # [OneBranch][Official] AKS Windows VHD Build EV2 Artifacts
    az pipelines variable update --name VHD_PIPELINE_RUN_ID --pipeline-id $pipeline_id --value $build_id  # Update the VHD_PIPELINE_RUN_ID with the build ID
}

something_to_do