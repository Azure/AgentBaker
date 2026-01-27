#!/bin/bash
set -euo pipefail
# Don't echo commands to the console, as this will cause Azure DevOps to do odd things with setvariable.
set +x

# This script packages the CSE for Windows into a zip file.

# check env variables CSE_PUBLISH_DIR, CSE_FILE_NAME are set
if [ -z "$CSE_RELEASE_DIR" ] || [ -z "$CSE_PUBLISH_DIR" ] || [ -z "$CSE_FILE_NAME" ]; then
  echo "required environment variables are not set. CSE_RELEASE_DIR=$CSE_RELEASE_DIR, CSE_PUBLISH_DIR=$CSE_PUBLISH_DIR, CSE_FILE_NAME=$CSE_FILE_NAME."
  exit 1
fi

echo "Creating CSE release pacakge $CSE_PUBLISH_DIR/$CSE_FILE_NAME"
mkdir -p "$CSE_RELEASE_DIR"
cp -r ./staging/cse/windows/* "$CSE_RELEASE_DIR"
rm "$CSE_RELEASE_DIR"/*.tests.ps1
rm "$CSE_RELEASE_DIR"/*.tests.suites -r
rm "$CSE_RELEASE_DIR"/README
rm "$CSE_RELEASE_DIR"/debug/update-scripts.ps1

mkdir -p "$CSE_PUBLISH_DIR"

pushd "$CSE_RELEASE_DIR" || exit 1
  zip -r "$CSE_PUBLISH_DIR/$CSE_FILE_NAME" ./*
popd || exit 1