#!/bin/bash
set -euxo pipefail
source vhdbuilder/scripts/automate_helpers.sh

EV2_ARTIFACT_PIPELINE_ID="319030" 
SIG_RELEASE_PIPELINE_ID="494"

VHD_BUILD_ID="${VHD_BUILD_ID:-""}"

trigger_ev2_artifacts() {
    echo "creating EV2 artifacts VHD build with ID: $VHD_BUILD_ID"

    # Run the pipeline and fetch the run ID to poll for success later
    EV2_BUILD_ID=$(az pipelines run --id $EV2_ARTIFACT_PIPELINE_ID --variables "VHD_PIPELINE_RUN_ID=$VHD_BUILD_ID" | jq -r '.id')
    if [ $? -ne 0 ]; then
        echo "failed to trigger EV2 artifact build"
        return 1
    fi
    while [ "$(az pipelines runs show --id $EV2_BUILD_ID | jq -r '.status')" == "inProgress" ]; do
        echo "EV2 artifact build $EV2_BUILD_ID is still running..."
        sleep 60
    done

    FINAL_STATUS="$(az pipelines runs show --id $EV2_BUILD_ID | jq -r '.status')"
    if [ "${FINAL_STATUS,,}" != "completed" ]; then
        echo "EV2 artifact build failed for VHD build with ID: $VHD_BUILD_ID, failed build ID: $EV2_BUILD_ID"
        return 1
    fi
    echo "EV2 artifacts successfully built for VHD build with ID: $VHD_BUILD_ID, EV2 build ID: $EV2_BUILD_ID"
}

create_release() {
    echo "creating SIG release for VHD with build ID: $VHD_BUILD_ID"

    RELEASE_URL=$(az pipelines release create --detect true \
        --project CloudNativeCompute \
        --definition-id $SIG_RELEASE_PIPELINE_ID \
        --artifact-metadata-list "ev2_artifacts=$EV2_BUILD_ID" | jq -r '.url')
    if [ $? -ne 0 ]; then
        echo "failed to create release for VHD with build ID: $VHD_BUILD_ID using artifacts from build: $EV2_BUILD_ID"
        return 1
    fi
    echo "SIG release successfully created for VHD build with ID: $VHD_BUILD_ID, release URL: $RELEASE_URL"
}

if [ -z "$VHD_BUILD_ID" ]; then
    echo "VHD_BUILD_ID must be set to run automated release trigger"
    exit 1
fi

retrycmd_if_failure 3 60 trigger_ev2_artifacts || exit $?
retrycmd_if_failure 3 60 create_release || exit $?
