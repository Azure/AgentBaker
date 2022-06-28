#!/usr/bin/env bash
set -uo pipefail
set -x

certSource=/opt/certs
certDestination=/usr/local/share/ca-certificates/certs

cp -a "$certSource"/. "$certDestination"

if [[ -z $(ls -A "$certSource") ]]; then
  ls "$certDestination" | grep -E '^[0-9]{14}' | while read -r line; do
    rm $certDestination/"$line"
  done
else
  certsToCopy=(${certSource}/*)
  currIterationCertFile=${certsToCopy[0]##*/}
  currIterationTag=${currIterationCertFile:0:14}
  for file in "$certDestination"/*.crt; do
      currFile=${file##*/}
     if [[ "${currFile:0:14}" != "${currIterationTag}" && -f "${file}" ]]; then
          rm "${file}"
     fi
  done
fi

update-ca-certificates -f