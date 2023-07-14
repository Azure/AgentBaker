#!/bin/bash
set -euxo pipefail

az login --identity

build_ids=$1

echo $1

echo $build_ids

trigger_pipeline() {
    #az pipelines run --id 188674 # --name "AKS Windows VHD Build -ContainerD&Docker"    
    #az pipelines run --id 182855 # use the AKS Windows VHD Build - PR check-in gate for trial and error
}

trigger_ev2_pipeline() {
    pipeline_id="322622" # [OneBranch][Official] AKS Windows VHD Build EV2 Artifacts
    for build_id in $build_ids; do
        echo "Build ID for the release is $build_id"
        az pipelines variable update --name VHD_PIPELINE_RUN_ID --pipeline-id $pipeline_id --value $build_id  # Update the VHD_PIPELINE_RUN_ID with the build ID
        
        # In case auth fails/other issue, we do not want the pipeline to run if the build ID was not correctly updated
        if [[ $build_id != $(az pipelines variable list --pipeline-id $pipeline_id | jq -r '.VHD_PIPELINE_RUN_ID.value') ]]; then
            echo "Build ID failed to update, cancel operation"
            exit 1
        else
            echo "Build ID successfully updated"
        fi

        # Run the pipeline and fetch the run ID to poll for success later
        run_id=$(az pipelines run --id $pipeline_id | jq -r '.id')
        while ! az pipelines runs show --id $run_id | grep -q '"result": "succeeded"'; do
            echo "ev2 artifacts still running for build $build_id"
            sleep 100
        done
        echo "ev2 artifacts successfully built for build $build_id"
    done
}
#trigger_pipeline

trigger_ev2_pipeline