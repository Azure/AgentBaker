#!/bin/bash
set -euxo pipefail

az login --identity

trigger_win_vhd_prod_pipeline() {
    #az pipelines run --id 188674 # --name "AKS Windows VHD Build -ContainerD&Docker"    
    az pipelines run --id 182855 # use the AKS Windows VHD Build - PR check-in gate for trial and error
}

queue_ev2_pipeline() {
    build_id="76289801"
    echo "Build ID for the release is $build_id"
    pipeline_id="322622" # [OneBranch][Official] AKS Windows VHD Build EV2 Artifacts
    az pipelines variable update --name VHD_PIPELINE_RUN_ID --pipeline-id $pipeline_id --value $build_id  # Update the VHD_PIPELINE_RUN_ID with the build ID
}

trigger_win_vhd_prod_pipeline
# queue_ev2_pipeline
