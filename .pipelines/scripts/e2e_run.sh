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
az login --identity --username "${E2E_AGENT_IDENTITY_ID}"
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

# Echo some variables so that we have a chance of debugging the pipeline if it fails due to a pipeline issue
echo "VHD_BUILD_ID=$VHD_BUILD_ID"
echo "IGNORE_SCENARIOS_WITH_MISSING_VHD: $IGNORE_SCENARIOS_WITH_MISSING_VHD"
echo "LOGGING_DIR: $LOGGING_DIR"
echo "E2E_SUBSCRIPTION_ID: ${E2E_SUBSCRIPTION_ID}"

# set variables that the go program expects if we are running a specific build
if [ -n "${VHD_BUILD_ID}" ]; then
  echo "VHD_BUILD_ID is specified (${VHD_BUILD_ID}). Running tests using VHDs from that build"
  export SIG_VERSION_TAG_NAME=buildId
  export SIG_VERSION_TAG_VALUE=$VHD_BUILD_ID
else
  echo "VHD_BUILD_ID is not specified. Running tests with default SIG version tag selectors."
fi

# this software is used to take the output of "go test" and produce a junit report that we can upload to the pipeline
# and see fancy test results.
cd e2e
mkdir -p bin
export GOBIN=`pwd`/bin/ go install github.com/jstemmer/go-junit-report/v2@latest

# Yes, we build first. That's because the exit code from "go test" below is eaten by the go-junit-report command. So if there are build problems
# then the tests pass. Bah.
go build -mod=readonly ./...

# Run the tests! Yey!
go test -v -parallel 100 -timeout 90m 2>&1 | ./bin/go-junit-report -iocopy -set-exit-code -out "${BUILD_SRC_DIR}/e2e/report.xml"
