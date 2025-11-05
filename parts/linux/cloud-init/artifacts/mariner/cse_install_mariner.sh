#!/bin/bash

removeContainerd() {
    containerdPackageName="containerd"
    if [ "$OS_VERSION" = "2.0" ]; then
        containerdPackageName="moby-containerd"
    fi
    retrycmd_if_failure 10 5 60 dnf remove -y $containerdPackageName
}

installDeps() {
    # The nftables package turns on a service by default that tries to load config files,
    # but the stock config files in the package have no uncommented lines and make the service
    # fail to start. Masking it as it's not used, and the stop action of "flush tables" can
    # result in rules getting cleared unexpectedly. Azure Linux 3 fixes this, so we only need
    # this in 2.0.
    if [ "$OS_VERSION" = "2.0" ]; then
      systemctl --now mask nftables.service || exit $ERR_SYSTEMCTL_MASK_FAIL
    fi

    # Install the package repo for the specific OS version.
    # AzureLinux 3.0 uses the azurelinux-repos-cloud-native repo
    # Other OS, e.g., Mariner 2.0 uses the mariner-repos-cloud-native repo
    if [ "$OS_VERSION" = "3.0" ]; then
      echo "Installing azurelinux-repos-cloud-native"
      dnf_install 30 1 600 azurelinux-repos-cloud-native
      dnf_install 30 1 600 azurelinux-repos-cloud-native-preview
    else
      echo "Installing mariner-repos-cloud-native"
      dnf_install 30 1 600 mariner-repos-cloud-native
    fi

    dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
    dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
    for dnf_package in ca-certificates check-restart cifs-utils cloud-init-azure-kvp conntrack-tools cracklib dnf-automatic ebtables ethtool fuse inotify-tools iotop iproute ipset iptables jq logrotate lsof nmap-ncat nfs-utils pam pigz psmisc rsyslog socat sysstat traceroute util-linux xz zip blobfuse2 nftables iscsi-initiator-utils device-mapper-multipath; do
      if ! dnf_install 30 1 600 $dnf_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done

    # install 2.0 specific packages
    # the blobfuse package is not available in AzureLinux 3.0
    if [ "$OS_VERSION" = "2.0" ]; then
      for dnf_package in apparmor-parser libapparmor blobfuse; do
        if ! dnf_install 30 1 600 $dnf_package; then
          exit $ERR_APT_INSTALL_TIMEOUT
        fi
      done
    fi

    # install apparmor related packages in AzureLinux 3.0
    # apparmor-utils is not installed in VHD as it brings auditd dependency
    # Only core AppArmor functionality (apparmor-parser, libapparmor) is included
    # Skip installation on CVM builds as they use different kernel configurations
    if [ "$OS_VERSION" = "3.0" ]; then
      # Check if this is a CVM build by inspecting FEATURE_FLAGS
      if echo "$FEATURE_FLAGS" | grep -q "cvm"; then
        echo "Skipping AppArmor installation on CVM build (FEATURE_FLAGS: $FEATURE_FLAGS)"
      else
        echo "Installing AppArmor packages for Azure Linux 3.0"
        for dnf_package in apparmor-parser libapparmor; do
          if ! dnf_install 30 1 600 $dnf_package; then
            exit $ERR_APT_INSTALL_TIMEOUT
          fi
        done
        systemctl enable apparmor.service
      fi
    fi
}

installKataDeps() {
    if [ "$OS_VERSION" != "1.0" ]; then
      if ! dnf_install 30 1 600 kata-packages-host; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    fi
}

installCriCtlPackage() {
  version="${1:-}"
  packageName="kubernetes-cri-tools-${version}"
  if [ -z "$version" ]; then
    echo "Error: No version specified for kubernetes-cri-tools package but it is required. Exiting with error."
  fi
  echo "Installing ${packageName} with dnf"
  dnf_install 30 1 600 ${packageName} || exit 1
}

downloadGPUDrivers() {
    # Mariner CUDA rpm name comes in the following format:
    #
    # 1. NVIDIA proprietary driver:
    # cuda-%{nvidia gpu driver version}_%{kernel source version}.%{kernel release version}.{mariner rpm postfix}
    #
    # 2. NVIDIA OpenRM driver:
    # cuda-open-%{nvidia gpu driver version}_%{kernel source version}.%{kernel release version}.{mariner rpm postfix}
    #
    # The proprietary driver will be used here in order to support older NVIDIA GPU SKUs like V100
    # Before installing cuda, check the active kernel version (uname -r) and use that to determine which cuda to install
    KERNEL_VERSION=$(uname -r | sed 's/-/./g')
    CUDA_PACKAGE=$(dnf repoquery -y --available "cuda*" | grep -E "cuda-[0-9]+.*_$KERNEL_VERSION" | sort -V | tail -n 1)

    if [ -z "$CUDA_PACKAGE" ]; then
      echo "No cuda packages found"
      exit $ERR_MISSING_CUDA_PACKAGE
    elif ! dnf_install 30 1 600 ${CUDA_PACKAGE}; then
      exit $ERR_APT_INSTALL_TIMEOUT
    fi
}

createNvidiaSymlinkToAllDeviceNodes() {
    NVIDIA_DEV_CHAR="/lib/udev/rules.d/71-nvidia-dev-char.rules"
    touch "${NVIDIA_DEV_CHAR}"
    cat << EOF > "${NVIDIA_DEV_CHAR}"
# This will create /dev/char symlinks to all device nodes
ACTION=="add", DEVPATH=="/bus/pci/drivers/nvidia", RUN+="/usr/bin/nvidia-ctk system create-dev-char-symlinks --create-all"
EOF

    /usr/bin/nvidia-ctk system create-dev-char-symlinks --create-all
}

installNvidiaFabricManager() {
    # Check the NVIDIA driver version installed and install nvidia-fabric-manager
    NVIDIA_DRIVER_VERSION=$(cut -d - -f 2 <<< "$(rpm -qa cuda)")
    for nvidia_package in nvidia-fabric-manager-${NVIDIA_DRIVER_VERSION} nvidia-fabric-manager-devel-${NVIDIA_DRIVER_VERSION}; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done
}

installNvidiaContainerToolkit() {
    MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION=$(jq -r '.Packages[] | select(.name == "nvidia-container-toolkit") | .downloadURIs.azurelinux.current.versionsV2[0].latestVersion' $COMPONENTS_FILEPATH)

    # Check if the version is empty and set the default if needed
    if [ -z "$MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION" ]; then
      echo "nvidia-container-toolkit not found in components.json" # Expected for older VHD with new CSE
      MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION="1.16.2"
    fi

    # The following packages need to be installed in this sequence because:
    # - libnvidia-container packages are required by nvidia-container-toolkit
    # - nvidia-container-toolkit-base provides nvidia-ctk that is used to generate the nvidia container runtime config
    #   during the posttrans phase of nvidia-container-toolkit package installation
    for nvidia_package in libnvidia-container1-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} libnvidia-container-tools-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} nvidia-container-toolkit-base-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION} nvidia-container-toolkit-${MARINER_NVIDIA_CONTAINER_TOOLKIT_VERSION}; do
      if ! dnf_install 30 1 600 $nvidia_package; then
        exit $ERR_APT_INSTALL_TIMEOUT
      fi
    done

}

enableNvidiaPersistenceMode() {
    PERSISTENCED_SERVICE_FILE_PATH="/etc/systemd/system/nvidia-persistenced.service"
    touch ${PERSISTENCED_SERVICE_FILE_PATH}
    cat << EOF > ${PERSISTENCED_SERVICE_FILE_PATH}
[Unit]
Description=NVIDIA Persistence Daemon
Wants=syslog.target

[Service]
Type=forking
ExecStart=/usr/bin/nvidia-persistenced --verbose
ExecStopPost=/bin/rm -rf /var/run/nvidia-persistenced
Restart=always

[Install]
WantedBy=multi-user.target
EOF

    systemctl enable nvidia-persistenced.service || exit 1
    systemctl restart nvidia-persistenced.service || exit 1
}

installCredentialProviderFromPMC() {
    k8sVersion="${1:-}"
    os=${AZURELINUX_OS_NAME}
    if [ -z "$OS_VERSION" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${OS_VERSION}"
    fi
   	PACKAGE_VERSION=""
    getLatestPkgVersionFromK8sVersion "$k8sVersion" "azure-acr-credential-provider-pmc" "$os" "$os_version"
    packageVersion=$(echo $PACKAGE_VERSION | cut -d "-" -f 1)
	echo "installing azure-acr-credential-provider package version: $packageVersion"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    installRPMPackageFromFile "azure-acr-credential-provider" "${packageVersion}" || exit $ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT
    mv "/usr/local/bin/azure-acr-credential-provider" "$CREDENTIAL_PROVIDER_BIN_DIR/acr-credential-provider"
}

installKubeletKubectlPkgFromPMC() {
    local desiredVersion="${1}"
	  installRPMPackageFromFile "kubelet" $desiredVersion || exit $ERR_KUBELET_INSTALL_FAIL
    installRPMPackageFromFile "kubectl" $desiredVersion || exit $ERR_KUBECTL_INSTALL_FAIL
}

installToolFromLocalRepo() {
    local tool_name=$1
    local tool_download_dir=$2

    # Verify the download directory exists and contains repository metadata
    if [ ! -d "${tool_download_dir}" ]; then
        echo "Download directory ${tool_download_dir} does not exist"
        return 1
    fi

    # Check if this is a self-contained local repository (has Packages.gz or repodata)
    if [ ! -d "${tool_download_dir}/repodata" ]; then
        echo "No valid repository metadata found in ${tool_download_dir}"
        return 1
    fi

    # Create a temporary repo configuration for the local directory
    local repo_name="local-${tool_name}-repo"
    local repo_file="/etc/yum.repos.d/${repo_name}.repo"

    echo "Setting up local repository from ${tool_download_dir}"

    # Create the repo file
    cat > "${repo_file}" <<EOF
[${repo_name}]
name=Local ${tool_name} Repository
baseurl=file://${tool_download_dir}
enabled=1
gpgcheck=0
skip_if_unavailable=1
EOF

    # Update DNF cache for the new repository
    echo "Updating DNF cache for local repository"
    dnf makecache --disablerepo='*' --enablerepo="${repo_name}" || {
        echo "Failed to update DNF cache for local repository"
        rm -f "${repo_file}"
        return 1
    }

    # Install the package from the local repository
    echo "Installing ${tool_name} from local repository"
    if ! dnf_install 30 1 600 ${tool_name} --disablerepo='*' --enablerepo="${repo_name}"; then
        echo "Failed to install ${tool_name} from local repository"
        rm -f "${repo_file}"
        return 1
    fi

    # Clean up the temporary repo file
    rm -f "${repo_file}"

    # Clean up the download directory
    rm -rf "${tool_download_dir}"

    echo "Successfully installed ${tool_name} from local repository"
    return 0
}

installCredentialProviderPackageFromBootstrapProfileRegistry() {
    bootstrapProfileRegistry="$1"
    k8sVersion="${2:-}"

    os=${AZURELINUX_OS_NAME}
    if [ -z "$OS_VERSION" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${OS_VERSION}"
    fi
    PACKAGE_VERSION=""
    getLatestPkgVersionFromK8sVersion "$k8sVersion" "azure-acr-credential-provider-pmc" "$os" "$os_version"
    packageVersion=$(echo $PACKAGE_VERSION | cut -d "-" -f 1)
    if [ -z "$packageVersion" ]; then
        packageVersion=$(echo "$CREDENTIAL_PROVIDER_DOWNLOAD_URL" | grep -oP 'v\d+(\.\d+)*' | sed 's/^v//' | head -n 1)
        if [ -z "$packageVersion" ]; then
            echo "Failed to determine package version for azure-acr-credential-provider"
            return $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        fi
    fi
    echo "installing azure-acr-credential-provider package version: $packageVersion"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    if ! installToolFromBootstrapProfileRegistry "azure-acr-credential-provider" $bootstrapProfileRegistry "${packageVersion}" "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider"; then
        if [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != "true" ] ; then
            # SHOULD_ENFORCE_KUBE_PMC_INSTALL will only be set for e2e tests, which should not fallback to reflect result of package installation behavior
            echo "Fall back to install credential provider from url installation"
            installCredentialProviderFromUrl
        else
            echo "Failed to install credential provider from bootstrap profile registry, and not falling back to package installation"
            exit $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        fi
    fi
}

updateDnfWithNvidiaPkg() {
  if [ "$OS_VERSION" != "3.0" ]; then
    echo "NVIDIA repo setup is only supported on Azure Linux 3.0"
    return
  fi

  local cpu_arch=$(getCPUArch) # Returns amd64 or arm64
  local repo_arch=""

  if [ "$cpu_arch" = "amd64" ]; then
    repo_arch="x86_64"
  elif [ "$cpu_arch" = "arm64" ]; then
    repo_arch="sbsa"
  else
    echo "Unsupported CPU architecture: $cpu_arch"
    return
  fi

  readonly nvidia_repo_path="/etc/yum.repos.d/nvidia-built-azurelinux.repo"
  local nvidia_repo_url="https://developer.download.nvidia.com/compute/cuda/repos/azl3/${repo_arch}/cuda-azl3.repo"
  retrycmd_curl_file 120 5 25 ${nvidia_repo_path} ${nvidia_repo_url} || exit $ERR_NVIDIA_AZURELINUX_REPO_FILE_DOWNLOAD_TIMEOUT
  dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
}

isPackageInstalled() {
    local packageName="${1}"
    if rpm -q "${packageName}" &>/dev/null; then
        return 0  # Package is installed
    else
        return 1  # Package is not installed
    fi
}

managedGPUPackageList() {
    packages=(
        nvidia-device-plugin
        datacenter-gpu-manager-4-core
        datacenter-gpu-manager-4-proprietary
        dcgm-exporter
    )
    echo "${packages[@]}"
}

installNvidiaManagedExpPkgFromCache() {
  if [ "$OS_VERSION" != "3.0" ]; then
    echo "Managed NVIDIA GPU experience is only supported on Azure Linux 3.0"
    return
  fi

  # Ensure kubelet device-plugins directory exists BEFORE package installation
  mkdir -p /var/lib/kubelet/device-plugins

  for packageName in $(managedGPUPackageList); do
    downloadDir="/opt/${packageName}/downloads"
    if isPackageInstalled "${packageName}"; then
      echo "${packageName} is already installed, skipping."
      rm -rf $(dirname ${downloadDir})
      continue
    fi

    rpmFile=$(find "${downloadDir}" -maxdepth 1 -name "${packageName}*" -print -quit 2>/dev/null) || rpmFile=""
    if [ -z "${rpmFile}" ]; then
      echo "Failed to locate ${packageName} rpm"
      exit $ERR_MANAGED_NVIDIA_EXP_INSTALL_FAIL
    fi

    logs_to_events "AKS.CSE.install${packageName}.dnf_install" "dnf_install 30 1 600 ${rpmFile}" || exit $ERR_APT_INSTALL_TIMEOUT
    rm -rf $(dirname ${downloadDir})
  done
}

installRPMPackageFromFile() {
    local packageName="${1}"
    local desiredVersion="${2}"
    echo "installing ${packageName} version ${desiredVersion}"
    downloadDir="/opt/${packageName}/downloads"
    packagePrefix="${packageName}-${desiredVersion}-*"

    rpmFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || rpmFile=""
    if [ -z "${rpmFile}" ]; then
        # query all package versions and get the latest version for matching k8s version
        fullPackageVersion=$(dnf list ${packageName} --showduplicates | grep ${desiredVersion}- | awk '{print $2}' | sort -V | tail -n 1)
        if [ -z "${fullPackageVersion}" ]; then
            echo "Failed to find valid ${packageName} version for ${desiredVersion}"
            exit 1
        fi
        echo "Did not find cached rpm file, downloading ${packageName} version ${fullPackageVersion}"
        downloadPkgFromVersion "${packageName}" ${fullPackageVersion} "${downloadDir}"
        rpmFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || rpmFile=""
    fi
	  if [ -z "${rpmFile}" ]; then
        echo "Failed to locate ${packageName} rpm"
        exit 1
    fi

    if ! dnf_install 30 1 600 ${rpmFile}; then
        exit $ERR_APT_INSTALL_TIMEOUT
    fi
    mv "/usr/bin/${packageName}" "/usr/local/bin/${packageName}"
	rm -rf ${downloadDir}
}

downloadPkgFromVersion() {
    packageName="${1:-}"
    packageVersion="${2:-}"
    downloadDir="${3:-"/opt/${packageName}/downloads"}"
    mkdir -p ${downloadDir}
    dnf_download 30 1 600 ${downloadDir} ${packageName}-${packageVersion} || exit $ERR_APT_INSTALL_TIMEOUT
    echo "Succeeded to download ${packageName} version ${packageVersion}"
}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    local desiredVersion="${1:-}"
    #e.g., desiredVersion will look like this 1.6.26-5.cm2
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    # check if containerd command is available before running it
    if command -v containerd &> /dev/null; then
        CURRENT_VERSION=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    fi
    # v1.4.1 is our lowest supported version of containerd
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${desiredVersion}; then
        echo "currently installed containerd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${desiredVersion}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${desiredVersion}"
        removeContainerd
        containerdPackageName="containerd-${desiredVersion}"
        if [ "$OS_VERSION" = "2.0" ]; then
            containerdPackageName="moby-containerd-${desiredVersion}"
        fi
        if [ "$OS_VERSION" = "3.0" ]; then
            containerdPackageName="containerd2-${desiredVersion}"
        fi

        # TODO: tie runc to r92 once that's possible on Mariner's pkg repo and if we're still using v1.linux shim
        if ! dnf_install 30 1 600 $containerdPackageName; then
            exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
    fi

    # Workaround to restore the CSE configuration after containerd has been installed from the package server.
    if [ -f /etc/containerd/config.toml.rpmsave ]; then
        mv /etc/containerd/config.toml.rpmsave /etc/containerd/config.toml
    fi

}

ensureRunc() {
  echo "Mariner Runc is included in the Mariner base image or containerd installation. Skipping downloading and installing Runc"
}

cleanUpGPUDrivers() {
  rm -Rf $GPU_DEST /opt/gpu

  for packageName in $(managedGPUPackageList); do
    rm -rf "/opt/${packageName}"
  done
}

downloadContainerdFromVersion() {
    echo "downloadContainerdFromVersion not implemented for mariner"
}

downloadContainerdFromURL() {
    echo "downloadContainerdFromURL not implemented for mariner"
}

#EOF
