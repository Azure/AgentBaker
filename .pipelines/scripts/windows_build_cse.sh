#!/bin/bash
set -uo pipefail

# This script builds CSE package in zip format. It has the following steps:
# 1. copy the cse pacakges to a temp directory
# 2. remove unnecessary files from the cse packages
# 3. zip it with name "aks-windows-cse-scripts-current.zip"
# 4. publish as an artifact


echo "Building CSE package for Windows..."
releaseTempDir=$(mktemp -d)
stagingDir=$(mkdir "$releaseTempDir/windows")

cp -r staging/cse/windows/* "$stagingDir"
rm "$stagingDir"/*.tests.ps1
rm "$stagingDir"/*.tests.suites -r
rm "$stagingDir"/README
rm "$stagingDir"/provisioningscripts/*.md
rm "$stagingDir"/debug/update-scripts.ps1

# Create a zip file of the CSE package
csePackage="aks-windows-cse-scripts-current.zip"
echo "Creating CSE elease pacakge $releaseTempDir/$csePackage"
pushd "$stagingDir" || exit 1
  zip -r "$releaseTempDir"/$csePackage ./*
popd || exit 1

# Publish the zip file as an artifact of the pipeline 
echo "Publishing CSE package as an artifact..."
echo "##vso[artifact.upload artifactname=CSEPackage;]$csePackage"
echo "CSE package built and published successfully."

rm -rf "$releaseTempDir"