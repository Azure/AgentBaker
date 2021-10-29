#!/bin/bash
set -x

source vhdbuilder/scripts/automate_helpers.sh

echo "Triggering ev2 artifact pipeline with Build ID $1"

build_id=$1

set +x
system_access_token=$2

trigger_pipeline() {
    az pipelines variable update --name VHD_PIPELINE_RUN_ID --pipeline-id 165939 --value $build_id  # Update the VHD_PIPELINE_RUN_ID with the build ID
    az pipelines run --id 165939  # 165939 is the pipeline ID for ev2 artifacts pipeline
}

configure_az_devops $system_access_token
set -x

trigger_pipeline