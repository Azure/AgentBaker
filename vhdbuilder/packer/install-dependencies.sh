#!/bin/bash

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"

#the following sed removes all comments of the format {{/* */}}
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/provision_source.sh
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/tool_installs_distro.sh

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh
source /home/packer/packer_source.sh

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
MANIFEST_FILEPATH=/opt/azure/manifest.json
KUBE_PROXY_IMAGES_FILEPATH=/opt/azure/kube-proxy-images.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
cat ${THIS_DIR}/kube-proxy-images.json > ${KUBE_PROXY_IMAGES_FILEPATH}
echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

if [[ $OS == $MARINER_OS_NAME ]]; then
  chmod 755 /opt
  chmod 755 /opt/azure
  chmod 644 ${VHD_LOGS_FILEPATH}
fi

copyPackerFiles
systemctlEnableAndStart disk_queue || exit 1

mkdir /opt/certs
chmod 666 /opt/certs
systemctlEnableAndStart update_certs.path || exit 1
systemctlEnableAndStart update_certs.timer || exit 1

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}

installDeps
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - apache2-utils
  - apt-transport-https
  - blobfuse=1.3.7
  - ca-certificates
  - ceph-common
  - cgroup-lite
  - cifs-utils
  - conntrack
  - cracklib-runtime
  - ebtables
  - ethtool
  - fuse
  - git
  - glusterfs-client
  - init-system-helpers
  - iproute2
  - ipset
  - iptables
  - jq
  - libpam-pwquality
  - libpwquality-tools
  - mount
  - nfs-common
  - pigz socat
  - traceroute
  - util-linux
  - xz-utils
  - netcat
  - dnsutils
  - zip
EOF

if [[ $(isARM64) == 1 ]]; then
  if [[ ${ENABLE_FIPS,,} == "true" ]]; then
    echo "No FIPS support on arm64, exiting..."
    exit 1
  fi
  if [[ $OS == $MARINER_OS_NAME ]]; then
    echo "No arm64 support for Mariner OS, exiting..."
    exit 1
  fi
  if [[ ${HYPERV_GENERATION,,} == "v1" ]]; then
    echo "No arm64 support on V1 VM, exiting..."
    exit 1
  fi

  if [[ ${CONTAINER_RUNTIME,,} == "docker" ]]; then
    echo "No dockerd is allowed on arm64 vhd, exiting..."
    exit 1
  fi

  # 5.4.0-1077-azure in the arm64 18.04 ubuntu image available on marketplace (Canonical:0002-com-ubuntu-server-arm-preview-bionic:18_04-lts:18.04.202204240)
  # misses many kernel configurations on which AKS doesn't work well. Canonical built a new kernel 5.4.0-1078-azure with proper kernel settings and shared in
  # the proposed repo. Canonical will publish new image containing 5.4.0-1078-azure kernel soon. For now we explicitly install this kernel. After new image is
  # available, we can remove this block.
  kernelVersion=$(uname -r)
  if [[ "${kernelVersion}" < "5.4.0-1078-azure" ]]; then
cat <<EOF >/etc/apt/sources.list.d/ubuntu-$(lsb_release -cs)-proposed.list
# Enable Ubuntu proposed archive
deb http://ports.ubuntu.com/ubuntu-ports $(lsb_release -cs)-proposed restricted main multiverse universe
EOF
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    retrycmd_if_failure 30 5 3600 apt-get install -y linux-image-5.4.0-1078-azure || exit $ERR_APT_INSTALL_TIMEOUT
    rm -f /etc/apt/sources.list.d/ubuntu-bionic-proposed.list
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
  fi
fi

if [[ ${UBUNTU_RELEASE} == "18.04" && ${ENABLE_FIPS,,} == "true" ]]; then
  installFIPS
elif [[ ${ENABLE_FIPS,,} == "true" ]]; then
  echo "AKS enables FIPS on Ubuntu 18.04 only, exiting..."
  exit 1
fi

if [[ ${UBUNTU_RELEASE} == "18.04" || ${UBUNTU_RELEASE} == "20.04" ]]; then
  overrideNetworkConfig || exit 1
  disableNtpAndTimesyncdInstallChrony || exit 1
fi

if [[ $OS == $MARINER_OS_NAME ]]; then
    disableSystemdResolvedCache
    disableSystemdIptables
    forceEnableIpForward
    networkdWorkaround
    enableDNFAutomatic
    fixCBLMarinerPermissions
    overrideNetworkConfig || exit 1
fi

downloadKrustlet
echo "  - krustlet ${KRUSTLET_VERSION}" >> ${VHD_LOGS_FILEPATH}

if [[ ${CONTAINER_RUNTIME:-""} == "containerd" ]]; then
  echo "VHD will be built with containerd as the container runtime"
  containerd_manifest="$(jq .containerd manifest.json)" || exit $?
  containerd_versions="$(echo ${containerd_manifest} | jq -r '.versions[]')" || exit $?

  for version in $containerd_versions; do
    containerd_version="$(echo "$version" | cut -d- -f1)"
    containerd_patch_version="$(echo "$version" | cut -d- -f2)"
    downloadContainerdFromVersion ${containerd_version} ${containerd_patch_version}
    echo "  - [cached] containerd v${containerd_version}-${containerd_patch_version}" >> ${VHD_LOGS_FILEPATH}
  done

  installed_version="$(echo ${containerd_manifest} | jq -r '.latest')"
  containerd_version="$(echo "$installed_version" | cut -d- -f1)"
  containerd_patch_version="$(echo "$installed_version" | cut -d- -f2)"
  installStandaloneContainerd ${containerd_version} ${containerd_patch_version}
  echo "  - [installed] containerd v${containerd_version}-${containerd_patch_version}" >> ${VHD_LOGS_FILEPATH}

  CRICTL_VERSIONS="
  1.19.0
  1.20.0
  1.21.0
  1.22.0
  1.23.0
  "
  for CRICTL_VERSION in ${CRICTL_VERSIONS}; do
    downloadCrictl ${CRICTL_VERSION}
    echo "  - crictl version ${CRICTL_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
  # k8s will use images in the k8s.io namespaces - create it
  ctr namespace create k8s.io
  cliTool="ctr"

  # also pre-download Teleportd plugin for containerd
  downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} "0.8.0"
else
  CONTAINER_RUNTIME="docker"
  MOBY_VERSION="19.03.14"
  installMoby
  echo "VHD will be built with docker as container runtime"
  echo "  - moby v${MOBY_VERSION}" >> ${VHD_LOGS_FILEPATH}
  cliTool="docker"
fi

INSTALLED_RUNC_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
echo "  - runc version ${INSTALLED_RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}

## for ubuntu-based images, cache multiple versions of runc
if [[ $OS == $UBUNTU_OS_NAME ]]; then
  RUNC_VERSIONS="
  1.0.0-rc92
  1.0.0-rc95
  1.0.3
  "
  if [[ $(isARM64) == 1 ]]; then
    # RUNC versions of 1.0.3 later might not be available in Ubuntu AMD64/ARM64 repo at the same time
    # so use different version set for different arch to avoid affecting each other during VHD build
    RUNC_VERSIONS="
    1.0.3
    "
  fi
  for RUNC_VERSION in $RUNC_VERSIONS; do
    downloadDebPkgToFile "moby-runc" ${RUNC_VERSION/\-/\~} ${RUNC_DOWNLOADS_DIR}
    echo "  - [cached] runc ${RUNC_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
fi

installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
addNvidiaAptRepo
installNvidiaDocker "${NVIDIA_DOCKER_VERSION}"
downloadGPUDrivers
retrycmd_if_failure 30 5 3600 wget "https://developer.download.nvidia.com/compute/cuda/redist/fabricmanager/linux-x86_64/fabricmanager-linux-x86_64-${GPU_DV}.tar.gz"
tar -xvzf fabricmanager-linux-x86_64-${GPU_DV}.tar.gz -C /opt/azure
mv /opt/azure/fabricmanager /opt/azure/fabricmanager-${GPU_DV}
echo "  - nvidia-docker2 nvidia-container-runtime" >> ${VHD_LOGS_FILEPATH}
retrycmd_if_failure 30 5 3600 apt-get -o Dpkg::Options::="--force-confold" install -y nvidia-container-runtime="${NVIDIA_CONTAINER_RUNTIME_VERSION}" --download-only || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
{
  echo "  - nvidia-container-runtime=${NVIDIA_CONTAINER_RUNTIME_VERSION}";
  echo "  - nvidia-gpu-driver-version=${GPU_DV}";
  echo "  - nvidia-fabricmanager=${GPU_DV}";
} >> ${VHD_LOGS_FILEPATH}
if grep -q "fullgpu" <<< "$FEATURE_FLAGS"; then
    echo "  - ensureGPUDrivers" >> ${VHD_LOGS_FILEPATH}
    ensureGPUDrivers
fi
fi

installBcc
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - bcc-tools
  - libbcc-examples
EOF

echo "${CONTAINER_RUNTIME} images pre-pulled:" >> ${VHD_LOGS_FILEPATH}

string_replace() {
  echo ${1//\*/$2}
}

ContainerImages=$(jq ".ContainerImages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
for imageToBePulled in ${ContainerImages[*]}; do
  downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
  amd64OnlyVersionsStr=$(echo "${imageToBePulled}" | jq .amd64OnlyVersions -r)
  multiArchVersionsStr=$(echo "${imageToBePulled}" | jq .multiArchVersions -r)

  amd64OnlyVersions=""
  if [[ ${amd64OnlyVersionsStr} != null ]]; then
    amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
  fi
  multiArchVersions=""
  if [[ ${multiArchVersionsStr} != null ]]; then
    multiArchVersions=$(echo "${multiArchVersionsStr}" | jq -r ".[]")
  fi

  if [[ $(isARM64) == 1 ]]; then
    versions="${multiArchVersions}"
  else
    versions="${amd64OnlyVersions} ${multiArchVersions}"
  fi

  for version in ${versions}; do
    CONTAINER_IMAGE=$(string_replace $downloadURL $version)
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
  done
done

watcher=$(jq '.ContainerImages[] | select(.downloadURL | contains("aks-node-ca-watcher"))' $COMPONENTS_FILEPATH)
watcherBaseImg=$(echo $watcher | jq -r .downloadURL)
watcherVersion=$(echo $watcher | jq -r .multiArchVersions[0])
watcherFullImg=${watcherBaseImg//\*/$watcherVersion}

# this image will never get pulled, the tag must be the same across different SHAs.
# it will only ever be upgraded via node image changes.
# we do this because the image is used to bootstrap custom CA trust when MCR egress
# may be intercepted by an untrusted TLS MITM firewall.
watcherStaticImg=${watcherBaseImg//\*/static}

# can't use cliTool because crictl doesn't support retagging.
if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    retagContainerImage "ctr" ${watcherFullImg} ${watcherStaticImg}
else
    retagContainerImage "docker" ${watcherFullImg} ${watcherStaticImg}
fi

#Azure CNI has binaries and container images for ARM64 from 1.4.13
AMD64_ONLY_CNI_VERSIONS="
1.2.7
"
#Please add new version (>=1.4.12) in this section in order that it can be pulled by both AMD64/ARM64 vhd
MULTI_ARCH_VNET_CNI_VERSIONS="
1.4.14
1.4.21
1.4.22
"

if [[ $(isARM64) == 1 ]]; then
  ARM64_VNET_CNI_VERSIONS_IN_USE="
  1.4.14
  "
  VNET_CNI_VERSIONS="${ARM64_VNET_CNI_VERSIONS_IN_USE} ${MULTI_ARCH_VNET_CNI_VERSIONS}"
else
  VNET_CNI_VERSIONS="${AMD64_ONLY_CNI_VERSIONS} ${MULTI_ARCH_VNET_CNI_VERSIONS}"
fi


for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

# merge with above after two more version releases
#Azure SWIFT CNI has binaries and container images for ARM64 from 1.4.13
AMD64_ONLY_SWIFT_CNI_VERSIONS="
1.2.7
1.4.12
"
#Please add new version (>=1.4.13) in this section in order that it can be pulled by both AMD64/ARM64 vhd
MULTI_ARCH_SWIFT_CNI_VERSIONS="
1.4.21
1.4.22
"

if [[ $(isARM64) == 1 ]]; then
  ARM64_SWIFT_CNI_VERSIONS_IN_USE="
  1.4.12
  "
  SWIFT_CNI_VERSIONS="${ARM64_SWIFT_CNI_VERSIONS_IN_USE} ${MULTI_ARCH_SWIFT_CNI_VERSIONS}"
else
  SWIFT_CNI_VERSIONS="${AMD64_ONLY_SWIFT_CNI_VERSIONS} ${MULTI_ARCH_SWIFT_CNI_VERSIONS}"
fi

for VNET_CNI_VERSION in $SWIFT_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-swift-linux-${CPU_ARCH}-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure Swift CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

if [[ $(isARM64) != 1 ]]; then  #v0.7.6 has no ARM64 binaries
  CNI_PLUGIN_VERSIONS="
  0.7.6
  "
  for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
fi

# After v0.7.6, URI was changed to renamed to https://acs-mirror.azureedge.net/cni-plugins/v*/binaries/cni-plugins-linux-arm64-v*.tgz
MULTI_ARCH_CNI_PLUGIN_VERSIONS="
0.9.1
"

if [[ $(isARM64) == 1 ]]; then
  ARM64_CNI_PLUGIN_VERSIONS_IN_USE="
  0.8.7
  "
  CNI_PLUGIN_VERSIONS="${ARM64_CNI_PLUGIN_VERSIONS_IN_USE} ${MULTI_ARCH_CNI_PLUGIN_VERSIONS}"
else
  CNI_PLUGIN_VERSIONS="${MULTI_ARCH_CNI_PLUGIN_VERSIONS}"
fi

for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni-plugins/v${CNI_PLUGIN_VERSION}/binaries/cni-plugins-linux-${CPU_ARCH}-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

if [[ $OS == $UBUNTU_OS_NAME && $(isARM64) != 1 ]]; then  # no ARM64 SKU with GPU now
NVIDIA_DEVICE_PLUGIN_VERSIONS="
v0.9.0
"
for NVIDIA_DEVICE_PLUGIN_VERSION in ${NVIDIA_DEVICE_PLUGIN_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/nvidia/k8s-device-plugin:${NVIDIA_DEVICE_PLUGIN_VERSION}"
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# GPU device plugin
if grep -q "fullgpu" <<< "$FEATURE_FLAGS" && grep -q "gpudaemon" <<< "$FEATURE_FLAGS"; then
  kubeletDevicePluginPath="/var/lib/kubelet/device-plugins"
  mkdir -p $kubeletDevicePluginPath
  echo "  - $kubeletDevicePluginPath" >> ${VHD_LOGS_FILEPATH}

  DEST="/usr/local/nvidia/bin"
  mkdir -p $DEST
  if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
    ctr --namespace k8s.io run --rm --mount type=bind,src=${DEST},dst=${DEST},options=bind:rw --cwd ${DEST} "mcr.microsoft.com/oss/nvidia/k8s-device-plugin:v0.9.0" plugingextract /bin/sh -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
  else
    docker run --rm --entrypoint "" -v $DEST:$DEST "mcr.microsoft.com/oss/nvidia/k8s-device-plugin:v0.9.0" /bin/bash -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
  fi
  chmod a+x $DEST/nvidia-device-plugin
  echo "  - extracted nvidia-device-plugin..." >> ${VHD_LOGS_FILEPATH}
  ls -ltr $DEST >> ${VHD_LOGS_FILEPATH}

  systemctlEnableAndStart nvidia-device-plugin || exit 1
  pushd /opt/azure/fabricmanager-${GPU_DV} || exit
  /opt/azure/fabricmanager-${GPU_DV}/fm_run_package_installer.sh
  systemctlEnableAndStart nvidia-fabricmanager
  popd || exit
fi

installSGX=${SGX_INSTALL:-"False"}
if [[ ${installSGX} == "True" ]]; then
    SGX_DEVICE_PLUGIN_VERSIONS="1.0"
    for SGX_DEVICE_PLUGIN_VERSION in ${SGX_DEVICE_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-device-plugin:${SGX_DEVICE_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_PLUGIN_VERSIONS="
    0.2
    0.4
    "
    for SGX_PLUGIN_VERSION in ${SGX_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-plugin:${SGX_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_WEBHOOK_VERSIONS="
    0.6
    0.9
    "
    for SGX_WEBHOOK_VERSION in ${SGX_WEBHOOK_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-webhook:${SGX_WEBHOOK_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_QUOTE_HELPER_VERSIONS="2.0"
    for SGX_QUOTE_HELPER_VERSION in ${SGX_QUOTE_HELPER_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-attestation:${SGX_QUOTE_HELPER_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
fi
fi

NGINX_VERSIONS="1.13.12-alpine"
for NGINX_VERSION in ${NGINX_VERSIONS}; do
    if [[ $(isARM64) == 1 ]]; then
        CONTAINER_IMAGE="docker.io/library/nginx:${NGINX_VERSION}"  # nginx in MCR is not 'multi-arch', pull it from docker.io now, upsteam team is building 'multi-arch' nginx for MCR
    else
        CONTAINER_IMAGE="mcr.microsoft.com/oss/nginx/nginx:${NGINX_VERSION}"
    fi
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done

# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.19.11
# v1.19.12
# v1.19.13
# v1.20.7
# v1.20.8
# v1.20.9
# v1.21.1
# v1.21.2
# v1.22.1 (preview)
# v1.22.2 (preview)
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP.

# kube-proxy regular versions >=v1.17.0  hotfixes versions >= 20211009 are 'multi-arch'. All versions in kube-proxy-images.json are 'multi-arch' version now.
if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
  KUBE_PROXY_IMAGE_VERSIONS=$(jq -r '.containerdKubeProxyImages.ContainerImages[0].multiArchVersions[]' <"$THIS_DIR/kube-proxy-images.json")
else
  KUBE_PROXY_IMAGE_VERSIONS=$(jq -r '.dockerKubeProxyImages.ContainerImages[0].multiArchVersions[]' <"$THIS_DIR/kube-proxy-images.json")
fi

for KUBE_PROXY_IMAGE_VERSION in ${KUBE_PROXY_IMAGE_VERSIONS}; do
  # use kube-proxy as well
  CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kube-proxy:v${KUBE_PROXY_IMAGE_VERSION}"
  pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
  if [[ ${cliTool} == "docker" ]]; then
      docker run --rm --entrypoint "" ${CONTAINER_IMAGE} /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"
  else
      ctr --namespace k8s.io run --rm ${CONTAINER_IMAGE} checkTask /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"
  fi
  # shellcheck disable=SC2181
  if [[ $? != 0 ]]; then
  echo "Hyperkube contains nf_tables, exiting..."
  exit 99
  fi
  echo "  - ${CONTAINER_IMAGE}" >>${VHD_LOGS_FILEPATH}
done

# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
# Please do not use the .1 suffix, because that's only for the base image patches
# regular version >= v1.17.0 or hotfixes >= 20211009 has arm64 binaries. For versions with arm64, please add it blow
MULTI_ARCH_KUBE_BINARY_VERSIONS="
1.21.7-hotfix.20220204
1.21.9-hotfix.20220204
1.22.4-hotfix.20220201
1.22.6-hotfix.20220130
1.23.3-hotfix.20220401
1.23.4-hotfix.20220331
1.23.5-hotfix.20220331
1.24.0-beta.0
"

KUBE_BINARY_VERSIONS="${MULTI_ARCH_KUBE_BINARY_VERSIONS}"

for PATCHED_KUBE_BINARY_VERSION in ${KUBE_BINARY_VERSIONS}; do
  if (($(echo ${PATCHED_KUBE_BINARY_VERSION} | cut -d"." -f2) < 19)) && [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    echo "Only need to store k8s components >= 1.19 for containerd VHDs"
    continue
  fi
  KUBERNETES_VERSION=$(echo ${PATCHED_KUBE_BINARY_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
  extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-${CPU_ARCH}.tar.gz"
done

# shellcheck disable=SC2129
echo "kubelet/kubectl downloaded:" >> ${VHD_LOGS_FILEPATH}
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

# shellcheck disable=SC2010
ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH} 

echo -e "=== Installed Packages Begin\n$(listInstalledPackages)\n=== Installed Packages End" >> ${VHD_LOGS_FILEPATH}

echo "Disk usage:" >> ${VHD_LOGS_FILEPATH}
df -h >> ${VHD_LOGS_FILEPATH}
# warn at 75% space taken
[ -s $(df -P | grep '/dev/sda1' | awk '0+$5 >= 75 {print}') ] || echo "WARNING: 75% of /dev/sda1 is used" >> ${VHD_LOGS_FILEPATH}
# error at 99% space taken
[ -s $(df -P | grep '/dev/sda1' | awk '0+$5 >= 99 {print}') ] || exit 1

echo "Using kernel:" >> ${VHD_LOGS_FILEPATH}
tee -a ${VHD_LOGS_FILEPATH} < /proc/version
{
  echo "Install completed successfully on " $(date)
  echo "VSTS Build NUMBER: ${BUILD_NUMBER}"
  echo "VSTS Build ID: ${BUILD_ID}"
  echo "Commit: ${COMMIT}"
  echo "Ubuntu version: ${UBUNTU_RELEASE}"
  echo "Hyperv generation: ${HYPERV_GENERATION}"
  echo "Feature flags: ${FEATURE_FLAGS}"
  echo "Container runtime: ${CONTAINER_RUNTIME}"
  echo "FIPS enabled: ${ENABLE_FIPS}"
} >> ${VHD_LOGS_FILEPATH}

if [[ $(isARM64) != 1 ]]; then
  # no asc-baseline-1.0.0-35.arm64.deb
  installAscBaseline
fi

if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
  if [[ ${ENABLE_FIPS,,} == "true" || ${CPU_ARCH} == "arm64" ]]; then
    relinkResolvConf
  fi
fi

if [[ $OS == $UBUNTU_OS_NAME ]]; then
  # remove snapd, which is not used by container stack
  apt-get purge --auto-remove snapd -y
  # update message-of-the-day to start after multi-user.target
  # multi-user.target usually start at the end of the boot sequence
  sed -i 's/After=network-online.target/After=multi-user.target/g' /lib/systemd/system/motd-news.service
fi
