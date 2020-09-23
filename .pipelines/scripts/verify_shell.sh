#!/bin/bash
set -euo pipefail
#inspired by https://github.com/Azure/aks-engine/blob/master/scripts/validate-shell.sh
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

filesToCheck=$(find . -type f -name "*.sh" -not -path './parts/linux/cloud-init/artifacts/*' -not -path './pkg/agent/testdata/*' -not -path './vendor/*' -not -path './hack/tools/vendor/*')

echo "Running shellcheck..."

IGNORED="
SC1127
SC1009
SC1054
SC1056
SC1072
SC1073
SC1083
SC1090
SC1091
SC2004
SC2006
SC2015
SC2034
SC2046
SC2053
SC2068
SC2086
SC2128
SC2145
SC2154
SC2206
SC2153
"
shellcheck $(printf -- "-e %s " $IGNORED) $filesToCheck