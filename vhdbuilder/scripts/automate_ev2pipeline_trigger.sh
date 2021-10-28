#!/bin/bash
set -x

source vhdbuilder/scripts/automate_helpers.sh

echo "Triggering ev2 artifact pipeline with build id $1"

build_id=$1
system_access_token=$2

trigger_pipeline() {
    az pipelines variable update --name VHD_PIPELINE_RUN_ID --pipeline-id 165939 --value $build_id
    az pipelines run --id 165939
}

configure_az_devops $system_access_token
trigger_pipeline