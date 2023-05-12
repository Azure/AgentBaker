#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/automate_helpers.sh

echo "Triggering ev2 artifact pipeline with Build ID $1"

build_ids=$1

trigger_pipeline() {
    pipeline_id="317897"   # 317897 is the pipeline ID for ev2 artifacts pipeline
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

trigger_pipeline