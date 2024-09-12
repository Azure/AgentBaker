COMPONENTS_FILEPATH="parts/linux/cloud-init/artifacts/components.json"


updateRelease() {
    local package="$1"
    local os="$2"
    local osVersion="$3"
    RELEASE="current"
    local osVersionWithoutDot=$(echo "${osVersion}" | sed 's/\.//g')
    #For UBUNTU, if $osVersion is 18.04 and "r1804" is also defined in components.json, then $release is set to "r1804"
    #Similarly for 20.04 and 22.04. Otherwise $release is set to .current.
    #For MARINER/AZURELINUX, the release is always set to "current" now.
    if [[ $(echo "${package}" | jq ".downloadURIs.ubuntu.\"r${osVersionWithoutDot}\"") != "null" ]]; then
        RELEASE="\"r${osVersionWithoutDot}\""
    fi
}

updatePackageVersions() {
    local package="$1"
    local os="$2"
    local osVersion="$3"
    RELEASE="current"
    echo "executing updateRelease with package=${package}"
    echo "executing updateRelease with os=${os}, osversion=${osVersion}"
    updateRelease "${package}" "${os}" "${osVersion}"
    echo "After updateRelease, RELEASE is ${RELEASE}"
    local osLowerCase=$(echo "${os}" | tr '[:upper:]' '[:lower:]')
    echo "osLowerCase is ${osLowerCase}"
    PACKAGE_VERSIONS=()

    # if .downloadURIs.${osLowerCase} doesn't exist, it will get the versions from .downloadURIs.default.
    # Otherwise get the versions from .downloadURIs.${osLowerCase}
    echo "************executing jq .downloadURIs.${osLowerCase} on ${package}"
    if [[ $(echo "${package}" | jq ".downloadURIs.${osLowerCase}") == "null" ]]; then
        osLowerCase="default"
    fi

    # jq the versions from the package. If downloadURIs.$osLowerCase.$release.versionsV2 is not null, then get the versions from there.
    # Otherwise get the versions from .downloadURIs.$osLowerCase.$release.versions
    echo "executing jq .downloadURIs.${osLowerCase}.${RELEASE}.versionsV2 on ${package}"
    if [[ $(echo "${package}" | jq ".downloadURIs.${osLowerCase}.${RELEASE}.versionsV2") != "null" ]]; then
        echo "executing jq -r .downloadURIs.${osLowerCase}.${RELEASE}.versionsV2[] | select(.latestVersion != null) | .latestVersion on ${package}"
        local latestVersions=($(echo "${package}" | jq -r ".downloadURIs.${osLowerCase}.${RELEASE}.versionsV2[] | select(.latestVersion != null) | .latestVersion"))
        echo "executing jq -r .downloadURIs.${osLowerCase}.${RELEASE}.versionsV2[] | select(.previousLatestVersion != null) | .previousLatestVersion on ${package}"
        local previousLatestVersions=($(echo "${package}" | jq -r ".downloadURIs.${osLowerCase}.${RELEASE}.versionsV2[] | select(.previousLatestVersion != null) | .previousLatestVersion"))
        for version in "${latestVersions[@]}"; do
            PACKAGE_VERSIONS+=("${version}")
        done
        for version in "${previousLatestVersions[@]}"; do
            PACKAGE_VERSIONS+=("${version}")
        done
        return
    fi

    # Fallback to versions if versionsV2 is null
    local versions=($(echo "${package}" | jq -r ".downloadURIs.${os}.${RELEASE}.versions[]"))
    for version in "${versions[@]}"; do
        PACKAGE_VERSIONS+=("${version}")
    done
    return 0
}

packages=$(jq ".Packages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
# Iterate over each element in the packages array
while IFS= read -r p; do
  PACKAGE_VERSIONS=()
  os="UBUNTU"
  OS_VERSION="20.04"
  if [[ "${OS}" == "${MARINER_OS_NAME}" && "${IS_KATA}" == "true" ]]; then
    os=${MARINER_KATA_OS_NAME}
  fi
  updatePackageVersions ${p} ${os} ${OS_VERSION}
  echo "---------------Package versions: ${PACKAGE_VERSIONS[@]}"
done <<< "$packages"