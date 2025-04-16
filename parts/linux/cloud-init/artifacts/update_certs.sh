#!/usr/bin/env bash
set -uo pipefail

certSource=/opt/certs
certDestination="${1:-/usr/local/share/ca-certificates/certs}"
updateCmd="${2:-update-ca-certificates -f}"
destPrefix="aks-custom-"

[ ! -d "$certDestination" ] && mkdir "$certDestination"
for file in "$certSource"/*; do
  [ -f "$file" ] || continue
  cp -a -- "$file" "$certDestination/$destPrefix${file##*/}"
done

if [ -z "$(ls -A "$certSource" 2>/dev/null)" ]; then
  echo "Source dir "$certSource" was empty, attempting to remove cert files"
  ls "$certDestination" | grep -E '^'$destPrefix'[0-9]{14}' | while read -r line; do
    echo "removing "$line" in "$certDestination""
    rm $certDestination/"$line"
  done
else
  echo "found cert files in "$certSource""
  certsToCopy=(${certSource}/*)
  currIterationCertFile=${certsToCopy[0]##*/}
  currIterationTag=${currIterationCertFile:0:14}
  for file in "$certDestination/$destPrefix"*.crt; do
     currFile=${file##*/}
     # shellcheck disable=SC3010
     if [[ "${currFile:${#destPrefix}:14}" != "${currIterationTag}" && -f "${file}" ]]; then
          echo "removing "$file" in "$certDestination""
          rm "${file}"
     fi
  done
fi

$updateCmd