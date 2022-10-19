#!/usr/bin/env bash
set -uo pipefail

certSource=/opt/certs
certDestination=/usr/local/share/ca-certificates/certs

cp -a "$certSource"/. "$certDestination"

if [[ -z $(ls -A "$certSource") ]]; then
  echo "Source dir "$certSource" was empty, attempting to remove cert files"
  ls "$certDestination" | grep -E '^[0-9]{14}' | while read -r line; do
    echo "removing "$line" in "$certDestination""
    rm $certDestination/"$line"
  done
else
  echo "found cert files in "$certSource""
  certsToCopy=(${certSource}/*)
  currIterationCertFile=${certsToCopy[0]##*/}
  currIterationTag=${currIterationCertFile:0:14}
  for file in "$certDestination"/*.crt; do
      currFile=${file##*/}
     if [[ "${currFile:0:14}" != "${currIterationTag}" && -f "${file}" ]]; then
          echo "removing "$file" in "$certDestination""
          rm "${file}"
     fi
  done
fi

update-ca-certificates -f