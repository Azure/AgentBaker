#!/usr/bin/env bash

# if anything goes wrong, then abort.
set -euo pipefail

# This script runs the AgentBaker e2e tests for a VHD. It uses the following environment variables:
# * SUBSCRIPTION_ID: this variable contains the subscription to run the e2e tests in
# * SUBSCRIPTION_ID_OVERRIDE: optional. When non-empty it takes precedence over
#   SUBSCRIPTION_ID. This lets a calling pipeline override the subscription at runtime
#   (e.g. a queue-time variable) since compile-time template expressions cannot see such overrides.
# * DefaultWorkingDirectory: this variable contains the default working directory. Likely "." is sufficient
# * VHD_BUILD_ID - the build identifier for the pipeline. This is optional and if it is missing then the latest build from
#   the main branch is used.
# * IGNORE_SCENARIOS_WITH_MISSING_VHD: a true/false flag that indicates if the build should fail if the VHD is missing.
# * BUILD_SRC_DIR: the src directory for the repository. Probably the same as DefaultWorkingDirectory.
# * E2E_FAILED_TESTS_RETRY_COUNT: the number of times the runner retries failed scenarios. Defaults to 0.

# In addition, the e2e test framework reads a whole lot of environment variables.
# These are defined in: e2e/config/config.go

# Prefer an explicit subscription override (e.g. a queue-time variable passed by the calling
# pipeline) over the variable group default. This is resolved here at runtime because
# compile-time template expressions cannot see queue-time variable overrides.
SUBSCRIPTION_ID="${SUBSCRIPTION_ID:-}"
SUBSCRIPTION_ID_OVERRIDE="${SUBSCRIPTION_ID_OVERRIDE:-}"
if [ -n "${SUBSCRIPTION_ID_OVERRIDE}" ]; then
  SUBSCRIPTION_ID="${SUBSCRIPTION_ID_OVERRIDE}"
fi

SUBSCRIPTION_ID="${SUBSCRIPTION_ID:?SUBSCRIPTION_ID or SUBSCRIPTION_ID_OVERRIDE must be set}"
az account set -s "${SUBSCRIPTION_ID}"
echo "Using subscription ${SUBSCRIPTION_ID} for e2e tests"

# Setup go
GOPATH="$(go env GOPATH)"
export GOPATH
go version

# specify the logging directory so logs go to the right place
DefaultWorkingDirectory="${DefaultWorkingDirectory:?DefaultWorkingDirectory must be set}"
LOGGING_DIR="scenario-logs-$(date +%s)"
export LOGGING_DIR
echo "setting logging dir to $LOGGING_DIR"
# tell DevOps to set the variable so later pipeline steps can use it.

echo "##vso[task.setvariable variable=LOGGING_DIR]$LOGGING_DIR"
# make sure the logging directory exists
mkdir -p "${DefaultWorkingDirectory}/e2e/${LOGGING_DIR}"

# default any unbound required variables if necessary
VHD_BUILD_ID="${VHD_BUILD_ID:-}"
IGNORE_SCENARIOS_WITH_MISSING_VHD="${IGNORE_SCENARIOS_WITH_MISSING_VHD:-}"
LOGGING_DIR="${LOGGING_DIR:-}"
ENABLE_SECURE_TLS_BOOTSTRAPPING="${ENABLE_SECURE_TLS_BOOTSTRAPPING:-true}"
TAGS_TO_SKIP="${TAGS_TO_SKIP:-}"
TAGS_TO_RUN="${TAGS_TO_RUN:-}"
E2E_GO_TEST_TIMEOUT="${E2E_GO_TEST_TIMEOUT:-80m}"
E2E_FAILED_TESTS_RETRY_COUNT="${E2E_FAILED_TESTS_RETRY_COUNT:-0}"
GALLERY_NAME="${GALLERY_NAME:-}"
SIG_GALLERY_NAME="${SIG_GALLERY_NAME:-}"

case "${E2E_FAILED_TESTS_RETRY_COUNT}" in
  '' | *[!0-9]*)
    echo "##vso[task.logissue type=error]E2E_FAILED_TESTS_RETRY_COUNT must be a non-negative integer, got: ${E2E_FAILED_TESTS_RETRY_COUNT}" >&2
    exit 1
    ;;
esac

# echo some variables so that we have a chance of debugging if things fail due to a pipeline issue
echo "VHD_BUILD_ID: ${VHD_BUILD_ID}"
echo "IGNORE_SCENARIOS_WITH_MISSING_VHD: ${IGNORE_SCENARIOS_WITH_MISSING_VHD}"
echo "LOGGING_DIR: ${LOGGING_DIR}"
echo "SUBSCRIPTION_ID: ${SUBSCRIPTION_ID}"
echo "ENABLE_SECURE_TLS_BOOTSTRAPPING: ${ENABLE_SECURE_TLS_BOOTSTRAPPING}"
echo "TAGS_TO_SKIP: ${TAGS_TO_SKIP}"
echo "TAGS_TO_RUN: ${TAGS_TO_RUN}"
echo "GALLERY_NAME: ${GALLERY_NAME}"
echo "SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"
echo "E2E_GO_TEST_TIMEOUT: ${E2E_GO_TEST_TIMEOUT}"
echo "E2E_FAILED_TESTS_RETRY_COUNT: ${E2E_FAILED_TESTS_RETRY_COUNT}"

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

az extension add --name bastion

cd e2e
go test -count=1 ./...

exec go run ./cmd/e2e run \
  --parallel 60 \
  --suite-timeout "${E2E_GO_TEST_TIMEOUT}" \
  --retries "${E2E_FAILED_TESTS_RETRY_COUNT}" \
  --log-dir "${LOGGING_DIR}" \
  --junit-file "${BUILD_SRC_DIR}/e2e/report.xml" \
  --output grouped
