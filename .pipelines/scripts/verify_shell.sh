#!/bin/bash
set -euo pipefail

installed=$(which shellcheck 2>&1 >/dev/null)
if [[ ${installed} -ne 0 ]]; then
    echo "shellcheck not installed...trying to install."
    DISTRO=$(uname | tr "[:upper:]" "[:lower:]")
    if [[ "${DISTRO}" == "ubuntu" ]]; then
        apt-get install shellcheck
    elif [[ "${DISTRO}" == "darwin" ]]; then
        brew install cabal-install
    else 
        echo "distro ${DISTRO} not supported at this time. skipping shellcheck"
        return
    fi
fi

filesToCheck=$(find . -type f -name "*.sh" -not -path './parts/linux/cloud-init/artifacts/*')

echo "Running shellcheck..."

IGNORED="
SC2004
SC2015
SC2034
"
shellcheck $(printf -- "-e %s " $IGNORED) $filesToCheck