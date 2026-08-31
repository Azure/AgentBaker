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
# * E2E_FAILED_TESTS_RETRY_COUNT: the number of times gotestsum should retry failed tests. Defaults to 0.
# * SKIP_CURRENT_SOURCE_VHD_SCENARIOS: skips scenarios that require a VHD built from the current source.

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
SKIP_CURRENT_SOURCE_VHD_SCENARIOS="${SKIP_CURRENT_SOURCE_VHD_SCENARIOS:-false}"
E2E_GO_TEST_TIMEOUT="${E2E_GO_TEST_TIMEOUT:-80m}"
E2E_FAILED_TESTS_RETRY_COUNT="${E2E_FAILED_TESTS_RETRY_COUNT:-0}"
GALLERY_NAME="${GALLERY_NAME:-}"
SIG_GALLERY_NAME="${SIG_GALLERY_NAME:-}"
SIG_VERSION_TAG_NAME="${SIG_VERSION_TAG_NAME:-branch}"
SIG_VERSION_TAG_VALUE="${SIG_VERSION_TAG_VALUE:-refs/heads/main}"

# The calling pipeline owns the policy for scenarios that require a current-source VHD.
if [ "${SKIP_CURRENT_SOURCE_VHD_SCENARIOS,,}" = "true" ]; then
  TAGS_TO_SKIP="${TAGS_TO_SKIP:+${TAGS_TO_SKIP},}RequiresCurrentSourceVHD=true"
fi
export TAGS_TO_SKIP

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
echo "SKIP_CURRENT_SOURCE_VHD_SCENARIOS: ${SKIP_CURRENT_SOURCE_VHD_SCENARIOS}"
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

# this software is used to take the output of "go test" and produce a junit report that we can upload to the pipeline
# and see fancy test results.
cd e2e
mkdir -p bin
architecture=$(uname -m)

case "$architecture" in
  x86_64 | amd64) architecture="amd64" ;;
  aarch64 | arm64) architecture="arm64" ;;
  *)
    echo "Unsupported architecture: $architecture"
    exit 1
    ;;
esac

gotestsum_version="1.13.0"
gotestsum_archive="gotestsum_${gotestsum_version}_linux_${architecture}.tar.gz"
gotestsum_url="https://github.com/gotestyourself/gotestsum/releases/download/v${gotestsum_version}/${gotestsum_archive}"

temp_file="$(mktemp)"
curl --fail --silent --show-error --location --retry 5 --retry-delay 10 --retry-max-time 300 --retry-connrefused "$gotestsum_url" -o "$temp_file"
tar -xzf "$temp_file" -C bin
chmod +x bin/gotestsum
rm -f "$temp_file"

# gotestsum configure to only show logs for failed tests, json file for detailed logs
# Run the tests! Yey!
test_exit_code=0
rerun_fails=""
rerun_fails_report=""
set -- --format testdox --junitfile "${BUILD_SRC_DIR}/e2e/report.xml" --jsonfile "${BUILD_SRC_DIR}/e2e/test-log.json"
if [ "${E2E_FAILED_TESTS_RETRY_COUNT}" -gt 0 ]; then
  rerun_fails="${E2E_FAILED_TESTS_RETRY_COUNT}"
  rerun_fails_report="${BUILD_SRC_DIR}/e2e/rerun-fails-report.json"
  set -- "$@" "--rerun-fails=$rerun_fails" --packages=. "--rerun-fails-report=$rerun_fails_report" --debug
fi
./bin/gotestsum "$@" -- -parallel 60 -timeout "${E2E_GO_TEST_TIMEOUT}" || test_exit_code=$?

if [ -n "$rerun_fails_report" ] && [ -s "$rerun_fails_report" ]; then
  echo "gotestsum rerun-fails report:"
  cat "$rerun_fails_report"
  echo "##vso[artifact.upload containerfolder=test-results;artifactname=e2e-rerun-fails-report]$rerun_fails_report"
fi

# Upload test results as Azure DevOps artifacts
echo "##vso[artifact.upload containerfolder=test-results;artifactname=e2e-test-log]${BUILD_SRC_DIR}/e2e/test-log.json"

exit $test_exit_code
