#!/bin/bash
set -euo pipefail

template=$1
if [ "${ACL_CANDIDATE_IMAGE:-false}" = "true" ]; then
  workdir=$(mktemp -d)
  trap 'rm -rf "$workdir"' EXIT
  candidate_template="${workdir}/acl-candidate.json"
  jq '.builders[0].azure_tags["abe2e-ignore"] = "true"' "$template" > "$candidate_template"
  template=$candidate_template
  echo "Building ACL candidate with abe2e-ignore=true at creation"
fi

packer build -timestamp-ui -var-file=vhdbuilder/packer/settings.json "$template"
