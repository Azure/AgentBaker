#!/bin/bash
set -euxo pipefail

source vhdbuilder/scripts/windows/automate_helpers.sh

echo "Triggering ev2 artifact pipeline with Build ID $1"
