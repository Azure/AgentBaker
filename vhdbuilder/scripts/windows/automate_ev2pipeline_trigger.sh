#!/bin/bash
set -euxo pipefail

echo "Triggering windows ev2 artifact pipeline with Build ID 76289801"

build_id="76289801"
az login --identity

trigger_win_ev2_pipeline() {
    pipeline_id="322622" # [OneBranch][Official] AKS Windows VHD Build EV2 Artifacts
    echo "Build ID for the release is $build_id"
    az pipelines variable update --name VHD_PIPELINE_RUN_ID --pipeline-id $pipeline_id --value $build_id  # Update the VHD_PIPELINE_RUN_ID with the build ID
        
    # In case auth fails/other issue, we do not want the pipeline to run if the build ID was not correctly updated
    
}
trigger_win_ev2_pipeline