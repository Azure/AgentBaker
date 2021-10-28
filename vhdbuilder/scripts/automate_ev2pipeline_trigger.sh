#!/bin/bash
set -x

echo "Triggering ev2 artifact pipeline with build id $1"
build_id=$1
system_access_token=$2

configure_az_devops() {
    az extension add -n azure-devops
    echo ${system_access_token} | az devops login --organization=https://dev.azure.com/msazure
    az devops configure --defaults organization=https://dev.azure.com/msazure project=CloudNativeCompute
}

trigger_pipeline() {
    az pipelines variable update --name VHD_PIPELINE_RUN_ID --pipeline-id 165939 --value $build_id
    az pipelines run --id 165939
}

configure_az_devops
trigger_pipeline