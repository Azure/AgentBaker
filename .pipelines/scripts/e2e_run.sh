#!/usr/bin/env bash

# if anything goes wrong, then abort.
set -euo pipefail

# This script runs the AgentBaker e2e tests for a VHD. It uses the following environment variables:
# * E2E_AGENT_IDENTITY_ID: this variable contains the managed identity ID to log into azure with
# * E2E_SUBSCRIPTION_ID: this variable contains the subscription to run the e2e tests in
# * DefaultWorkingDirectory: this variable contains the default working directory. Likely "." is sufficient
# * VHD_BUILD_ID - the build identifier for the pipeline. This is optional and if it is missing then the latest build from
#   the main branch is used.
# * IGNORE_SCENARIOS_WITH_MISSING_VHD: a true/false flag that indicates if the build should fail if the VHD is missing.
# * BUILD_SRC_DIR: the src directory for the repository. Probably the same as DefaultWorkingDirectory.

# In addition, the e2e test framework reads a whole lot of environment variables.
# These are defined in: e2e/config/config.go

# First, login.
az login --identity --resource-id "${E2E_AGENT_IDENTITY_ID}"
az account set -s "${E2E_SUBSCRIPTION_ID}"
echo "Using subscription ${E2E_SUBSCRIPTION_ID} for e2e tests"

# Setup go
export GOPATH="$(go env GOPATH)"
go version

# specify the logging directory so logs go to the right place
export LOGGING_DIR="scenario-logs-$(date +%s)"
echo "setting logging dir to $LOGGING_DIR"
# tell DevOps to set the variable so later pipeline steps can use it.

echo "##vso[task.setvariable variable=LOGGING_DIR]$LOGGING_DIR"
# make sure the logging directory exists
mkdir -p "${DefaultWorkingDirectory}/e2e/${LOGGING_DIR}"

# default any unbound required variables if necessary
VHD_BUILD_ID="${VHD_BUILD_ID:-}"
IGNORE_SCENARIOS_WITH_MISSING_VHD="${IGNORE_SCENARIOS_WITH_MISSING_VHD:-}"
LOGGING_DIR="${LOGGING_DIR:-}"
E2E_SUBSCRIPTION_ID="${E2E_SUBSCRIPTION_ID:-}"
TAGS_TO_SKIP="${TAGS_TO_SKIP:-}"
TAGS_TO_RUN="${TAGS_TO_RUN:-}"
GALLERY_NAME="${GALLERY_NAME:-}"
SIG_GALLERY_NAME="${SIG_GALLERY_NAME:-}"

# echo some variables so that we have a chance of debugging if things fail due to a pipeline issue
echo "VHD_BUILD_ID: ${VHD_BUILD_ID}"
echo "IGNORE_SCENARIOS_WITH_MISSING_VHD: ${IGNORE_SCENARIOS_WITH_MISSING_VHD}"
echo "LOGGING_DIR: ${LOGGING_DIR}"
echo "E2E_SUBSCRIPTION_ID: ${E2E_SUBSCRIPTION_ID}"
echo "TAGS_TO_SKIP: ${TAGS_TO_SKIP}"
echo "TAGS_TO_RUN: ${TAGS_TO_RUN}"
echo "GALLERY_NAME: ${GALLERY_NAME}"
echo "SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"

# set variables that the go program expects if we are running a specific build
if [ -n "${VHD_BUILD_ID}" ]; then
  echo "VHD_BUILD_ID is specified (${VHD_BUILD_ID}). Running tests using VHDs from that build"
  export SIG_VERSION_TAG_NAME=buildId
  export SIG_VERSION_TAG_VALUE=$VHD_BUILD_ID
else
  echo "VHD_BUILD_ID is not specified. Running tests with default SIG version tag selectors."
fi

if [ -n "${SIG_GALLERY_NAME}" ]; then
  echo "SIG_GALLERY_NAME is specified (${SIG_GALLERY_NAME}). Updating GALLERY_NAME to $SIG_GALLERY_NAME"
  export GALLERY_NAME=$SIG_GALLERY_NAME
fi

# this software is used to take the output of "go test" and produce a junit report that we can upload to the pipeline
# and see fancy test results.
cd e2e
mkdir -p bin
GOBIN=`pwd`/bin/ go install gotest.tools/gotestsum@latest

# gotestsum configure to only show logs for failed tests, json file for detailed logs
# Run the tests! Yey!
test_exit_code=0
./bin/gotestsum --format testdox --junitfile "${BUILD_SRC_DIR}/e2e/report.xml" --jsonfile "${BUILD_SRC_DIR}/e2e/test-log.json" -- -parallel 100 -timeout 90m || test_exit_code=$?

# Upload test results as Azure DevOps artifacts
echo "##vso[artifact.upload containerfolder=test-results;artifactname=e2e-test-log]${BUILD_SRC_DIR}/e2e/test-log.json"

exit $test_exit_code
