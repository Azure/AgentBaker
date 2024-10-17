#!/bin/bash
set -euo pipefail
source vhdbuilder/scripts/automate_helpers.sh
set +x

EV2_ARTIFACT_PIPELINE_ID="319030" 
SIG_RELEASE_PIPELINE_ID="494"

RELEASE_EV2_ARTIFACTS_ALIAS_NAME="ev2_artifacts"
RELEASE_API_ENDPOINT="https://msazure.vsrm.visualstudio.com/CloudNativeCompute/_apis/Release/releases?api-version=5.1"

ADO_PAT="${ADO_PAT:-""}"
VHD_BUILD_ID="${VHD_BUILD_ID:-""}"

trigger_ev2_artifacts() {
    echo "creating EV2 artifacts for VHD build with ID: $VHD_BUILD_ID"

    RESPONSE=$(az pipelines run --id $EV2_ARTIFACT_PIPELINE_ID --variables "VHD_PIPELINE_RUN_ID=$VHD_BUILD_ID")
    EV2_BUILD_ID=$(echo "$RESPONSE" | jq -r '.id')
    EV2_BUILD_NUMBER=$(echo "$RESPONSE" | jq -r '.buildNumber')
    EV2_BUILD_URL="https://msazure.visualstudio.com/CloudNativeCompute/_build/results?buildId=${EV2_BUILD_ID}&view=results"
    STATUS="$(az pipelines runs show --id $EV2_BUILD_ID | jq -r '.status')"

    while [ "${STATUS,,}" == "notstarted" ] || [ "${STATUS,,}" == "inprogress" ]; do
        echo "EV2 artifact build $EV2_BUILD_ID is still in-progress..."
        sleep 30
        STATUS="$(az pipelines runs show --id $EV2_BUILD_ID | jq -r '.status')"
    done

    if [ "${STATUS,,}" != "completed" ]; then
        echo "EV2 artifact build finished with unknown status \"$STATUS\": $EV2_BUILD_URL"
        return 1
    fi

    RESULT="$(az pipelines runs show --id $EV2_BUILD_ID | jq -r '.result')"
    if [ "${RESULT,,}" == "failed" ]; then
        echo "EV2 artifact build for VHD build $VHD_BUILD_ID failed: $EV2_BUILD_URL"
        return 1
    fi

    if [ "${RESULT,,}" == "partiallysucceeded" ]; then
        echo "WARNING: EV2 artifact build for VHD build $VHD_BUILD_ID only partially succeeded: $EV2_BUILD_URL"
        echo "will still continue to create the release..."
        return 0
    fi

    echo "EV2 artifacts successfully built for VHD build: $VHD_BUILD_ID, EV2 build ID: $EV2_BUILD_ID"
}

create_release() {
    echo "creating SIG release for VHD with build ID: $VHD_BUILD_ID"

    # we do it this way for now since the devops CLI extension doesn't support what we need: https://github.com/Azure/azure-devops-cli-extension/issues/1257
    echo "sending POST request to $RELEASE_API_ENDPOINT"
    REQUEST_BODY="{'artifacts': [{'alias': '$RELEASE_EV2_ARTIFACTS_ALIAS_NAME', 'instanceReference': {'id': '$EV2_BUILD_ID', 'name': '$EV2_BUILD_NUMBER'}}], 'definitionId': $SIG_RELEASE_PIPELINE_ID}"
    RESPONSE=$(curl -X POST -H "Authorization: Basic $(echo -n ":$ADO_PAT" | base64)" -H "Content-Type: application/json" -d "$REQUEST_BODY" "$RELEASE_API_ENDPOINT")
    
    RELEASE_ID=$(echo "$RESPONSE" | jq -r '.id')
    if [ -z "$RELEASE_ID" ]; then
        echo "Error: SIG release not created successfully, unable to extract release ID"
        return 1
    fi
    echo "SIG release successfully created for VHD build with ID: $VHD_BUILD_ID"
    echo "release URL: https://msazure.visualstudio.com/CloudNativeCompute/_releaseProgress?_a=release-pipeline-progress&releaseId=$RELEASE_ID"

    # TODO(cameissner): extract approval link
}

if [ -z "$ADO_PAT" ]; then
    echo "ADO_PAT must be set to run automated EV2 artifact + release trigger"
    exit 1
fi

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set to run automated EV2 artifact + release trigger"
    exit 1
fi

retrycmd_if_failure 3 60 trigger_ev2_artifacts || exit $?
retrycmd_if_failure 3 60 create_release || exit $?
