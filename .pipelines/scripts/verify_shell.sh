#!/bin/bash
set -uo pipefail
#inspired by https://github.com/Azure/aks-engine/blob/master/scripts/validate-shell.sh
echo "Checking for shellcheck"
installed=$(command -v shellcheck 2>&1 >/dev/null; echo $?)

# must be set after above, or else `command -v` failure exits whole script.
set -e
if [[ "${installed}" -ne 0 ]]; then
    echo "shellcheck not installed...trying to install."
    DISTRO="$(uname | tr "[:upper:]" "[:lower:]")"
    # override for custom kernels, wsl, etc.
    if [[ "${DISTRO}" == "Linux" || "${DISTRO}" == "linux" ]]; then
        DISTRO="$(grep ^ID= < /etc/os-release | cut -d= -f2)"
    fi
    if [[ "${DISTRO}" == "ubuntu" ]]; then
        sudo apt-get install shellcheck -y
    elif [[ "${DISTRO}" == "darwin" ]]; then
        brew install cabal-install shellcheck
    else 
        echo "distro ${DISTRO} not supported at this time. skipping shellcheck"
        exit 1
    fi
else
    echo "shellcheck installed"
fi

filesToCheck=$(find . -type f -name "*.sh" -not -path './parts/linux/cloud-init/artifacts/*' -not -path './pkg/agent/testdata/*' -not -path './vendor/*' -not -path './hack/tools/vendor/*' -not -path './.git/*' -not -path './self-contained/*')

# also shell-check generated test data
generatedTestData=$(find ./pkg/agent/testdata -type f -name "*.sh" )
for file in $generatedTestData; do
    firstLine=$(awk 'NR==1 {print; exit}' ${file})
    if [[ ${firstLine} =~ "#!/bin/bash" ]]; then
        filesToCheck+=(${file})
    fi
done

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
SC2021
SC2128
SC2145
SC2154
SC2206
SC2153
SC2129
SC2286
SC2048
"
shellcheck $(printf -- "-e %s " $IGNORED) $filesToCheck
