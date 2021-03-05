#!/bin/bash
source /home/packer/provision_installs.sh
source /home/packer/provision_source.sh
source /home/packer/tool_installs.sh
source /home/packer/packer_source.sh

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

copyPackerFiles

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}

installDeps
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - apache2-utils
  - apt-transport-https
  - blobfuse=1.3.5
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
  - zip
EOF

if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
  overrideNetworkConfig || exit 1
  disableSystemdTimesyncdAndEnableNTP || exit 1
fi

if [[ ${CONTAINER_RUNTIME:-""} == "containerd" ]]; then
  echo "VHD will be built with containerd as the container runtime"
  CONTAINERD_VERSION="1.4.3"
  installStandaloneContainerd
  echo "  - containerd v${CONTAINERD_VERSION}" >> ${VHD_LOGS_FILEPATH}
  CRICTL_VERSIONS="1.19.0"
  for CRICTL_VERSION in ${CRICTL_VERSIONS}; do
    downloadCrictl ${CRICTL_VERSION}
    echo "  - crictl version ${CRICTL_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
  # k8s will use images in the k8s.io namespaces - create it
  ctr namespace create k8s.io
  cliTool="ctr"

  # also pre-download Teleportd plugin for containerd
  downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} "0.6.0"
else
  CONTAINER_RUNTIME="docker"
  MOBY_VERSION="19.03.14"
  installMoby
  echo "VHD will be built with docker as container runtime"
  echo "  - moby v${MOBY_VERSION}" >> ${VHD_LOGS_FILEPATH}
  cliTool="docker"
fi

installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

installGPUDrivers
echo "  - nvidia-docker2 nvidia-container-runtime" >> ${VHD_LOGS_FILEPATH}
retrycmd_if_failure 30 5 3600 apt-get -o Dpkg::Options::="--force-confold" install -y nvidia-container-runtime="${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" --download-only || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
echo "  - nvidia-container-runtime=${NVIDIA_CONTAINER_RUNTIME_VERSION}+docker18.09.2-1" >> ${VHD_LOGS_FILEPATH}

if grep -q "fullgpu" <<< "$FEATURE_FLAGS"; then
    echo "  - ensureGPUDrivers" >> ${VHD_LOGS_FILEPATH}
    ensureGPUDrivers
fi

installBcc
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - bcc-tools
  - libbcc-examples
EOF

installImg
echo "  - img" >> ${VHD_LOGS_FILEPATH}

echo "${CONTAINER_RUNTIME} images pre-pulled:" >> ${VHD_LOGS_FILEPATH}

string_replace() {
  echo ${1//\*/$2}
}

ContainerImages=$(jq ".ContainerImages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
for imageToBePulled in ${ContainerImages[*]}; do
  downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
  versions=$(echo "${imageToBePulled}" | jq .versions -r | jq -r ".[]")

  for version in ${versions}; do
    CONTAINER_IMAGE=$(string_replace $downloadURL $version)
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
  done
done

DownloadFiles=$(jq ".DownloadFiles" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)
for fileToDownload in ${DownloadFiles[*]}; do
  fileName=$(echo "${fileToDownload}" | jq .fileName -r)
  downloadLocation=$(echo "${fileToDownload}" | jq .downloadLocation -r)
  versions=$(echo "${fileToDownload}" | jq .versions -r | jq -r ".[]")
  download_URL=$(echo "${fileToDownload}" | jq .downloadURL -r)
  mkdir -p $downloadLocation

  for version in ${versions}; do
    file_Name=$(string_replace $fileName $version)
    dest="$downloadLocation/${file_Name}"
    downloadURL=$(string_replace $download_URL $version)/$file_Name
    retrycmd_get_tarball 120 5 ${dest} ${downloadURL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
  done
done

NVIDIA_DEVICE_PLUGIN_VERSIONS="
1.11
1.10
"
for NVIDIA_DEVICE_PLUGIN_VERSION in ${NVIDIA_DEVICE_PLUGIN_VERSIONS}; do
  if [[ "${cliTool}" == "ctr" ]]; then
    # containerd/ctr doesn't auto-resolve to docker.io
    CONTAINER_IMAGE="docker.io/nvidia/k8s-device-plugin:${NVIDIA_DEVICE_PLUGIN_VERSION}"
  else
    CONTAINER_IMAGE="nvidia/k8s-device-plugin:${NVIDIA_DEVICE_PLUGIN_VERSION}"
  fi
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
    ctr --namespace k8s.io run --rm --mount type=bind,src=${DEST},dst=${DEST},options=bind:rw --cwd ${DEST} "docker.io/nvidia/k8s-device-plugin:1.11" plugingextract /bin/sh -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1   
  else
    docker run --rm --entrypoint "" -v $DEST:$DEST "nvidia/k8s-device-plugin:1.11" /bin/bash -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
  fi
  chmod a+x $DEST/nvidia-device-plugin
  echo "  - extracted nvidia-device-plugin..." >> ${VHD_LOGS_FILEPATH}
  ls -ltr $DEST >> ${VHD_LOGS_FILEPATH}

  systemctlEnableAndStart nvidia-device-plugin || exit 1
fi

installSGX=${SGX_DEVICE_PLUGIN_INSTALL:-"False"}
if [[ ${installSGX} == "True" ]]; then
    SGX_DEVICE_PLUGIN_VERSIONS="1.0"
    for SGX_DEVICE_PLUGIN_VERSION in ${SGX_DEVICE_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-device-plugin:${SGX_DEVICE_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_PLUGIN_VERSIONS="0.1"
    for SGX_PLUGIN_VERSION in ${SGX_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-plugin:${SGX_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    SGX_WEBHOOK_VERSIONS="0.1"
    for SGX_WEBHOOK_VERSION in ${SGX_WEBHOOK_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-webhook:${SGX_WEBHOOK_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
fi

NGINX_VERSIONS="1.13.12-alpine"
for NGINX_VERSION in ${NGINX_VERSIONS}; do
    if [[ "${cliTool}" == "ctr" ]]; then
      # containerd/ctr doesn't auto-resolve to docker.io
      CONTAINER_IMAGE="docker.io/library/nginx:${NGINX_VERSION}"
    else
      CONTAINER_IMAGE="nginx:${NGINX_VERSION}"
    fi
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
done


# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.17.13
# v1.17.16
# v1.18.10
# v1.18.14
# v1.19.6
# v1.19.7
# v1.20.2
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
K8S_VERSIONS="
1.17.3-hotfix.20200601.1
1.17.7-hotfix.20200817.1
1.17.9-hotfix.20200824.1
1.17.11-hotfix.20200901.1
1.17.13
1.17.16
1.18.2-hotfix.20200624.1
1.18.4-hotfix.20200626.1
1.18.6-hotfix.20200723.1
1.18.8-hotfix.20200924
1.18.10-hotfix.20210118
1.18.14-hotfix.20210118
1.19.0
1.19.1-hotfix.20200923
1.19.3
1.19.6-hotfix.20210118
1.19.7-hotfix.20210122
1.20.2
"
for PATCHED_KUBERNETES_VERSION in ${K8S_VERSIONS}; do
  # Only need to store k8s components >= 1.19 for containerd VHDs
  if (($(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f2) < 19)) && [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    continue
  fi
  if (($(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f2) < 17)); then
    HYPERKUBE_URL="mcr.microsoft.com/oss/kubernetes/hyperkube:v${PATCHED_KUBERNETES_VERSION}"
    # NOTE: the KUBERNETES_VERSION will be used to tag the extracted kubelet/kubectl in /usr/local/bin
    # it should match the KUBERNETES_VERSION format(just version number, e.g. 1.15.7, no prefix v)
    # in installKubeletAndKubectl() executed by cse, otherwise cse will need to download the kubelet/kubectl again
    KUBERNETES_VERSION=$(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
    # extractHyperkube will extract the kubelet/kubectl binary from the image: ${HYPERKUBE_URL}
    # and put them to /usr/local/bin/kubelet-${KUBERNETES_VERSION}
    extractHyperkube ${cliTool}
    # remove hyperkube here as the one that we really need is pulled later
    removeContainerImage ${cliTool} $HYPERKUBE_URL
  else
    # strip the last .1 as that is for base image patch for hyperkube
    if grep -iq hotfix <<< ${PATCHED_KUBERNETES_VERSION}; then
      # shellcheck disable=SC2006
      PATCHED_KUBERNETES_VERSION=`echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f1,2,3,4`;
    else
      PATCHED_KUBERNETES_VERSION=`echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f1,2,3`;
    fi
    KUBERNETES_VERSION=$(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
    extractKubeBinaries $KUBERNETES_VERSION "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBERNETES_VERSION}/binaries/kubernetes-node-linux-amd64.tar.gz"
  fi
done
ls -ltr /usr/local/bin/* >> ${VHD_LOGS_FILEPATH}

# pull patched hyperkube image for AKS
# this is used by kube-proxy and need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.17.13
# v1.17.16
# v1.18.10
# v1.18.14
# v1.19.6
# v1.19.7
# v1.20.2
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP.
PATCHED_HYPERKUBE_IMAGES="
1.17.3-hotfix.20200601.1
1.17.7-hotfix.20200714.2
1.17.9-hotfix.20200824.1
1.17.11-hotfix.20200901
1.17.11-hotfix.20200901.1
1.17.13
1.17.16
1.18.4-hotfix.20200626.1
1.18.6-hotfix.20200723.1
1.18.8-hotfix.20200924
1.18.10-hotfix.20210118
1.18.14-hotfix.20210118
1.19.0
1.19.1-hotfix.20200923
1.19.3
1.19.6-hotfix.20210118
1.19.7-hotfix.20210122
1.20.2
"
for KUBERNETES_VERSION in ${PATCHED_HYPERKUBE_IMAGES}; do
  # Only need to store k8s components >= 1.19 for containerd VHDs
  if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) < 19)) && [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
    continue
  fi
  # TODO: after CCP chart is done, change below to get hyperkube only for versions less than 1.17 only
  if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) < 19)); then
      CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/hyperkube:v${KUBERNETES_VERSION}"
      pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
      echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
      if [[ ${cliTool} == "docker" ]]; then
          docker run --rm --entrypoint "" ${CONTAINER_IMAGE} /bin/sh -c "iptables --version" | grep -v nf_tables && echo "Hyperkube contains no nf_tables"
      else 
          ctr --namespace k8s.io run --rm ${CONTAINER_IMAGE} checkTask /bin/sh -c "iptables --version" | grep -v nf_tables && echo "Hyperkube contains no nf_tables"
      fi
      # shellcheck disable=SC2181
      if [[ $? != 0 ]]; then
      echo "Hyperkube contains nf_tables, exiting..."
      exit 99
      fi
  fi

  # from 1.17 onwards start using kube-proxy as well
  # strip the last .1 as that is for base image patch for hyperkube
  if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)); then
      if grep -iq hotfix <<< ${KUBERNETES_VERSION}; then
      KUBERNETES_VERSION=`echo ${KUBERNETES_VERSION} | cut -d"." -f1,2,3,4`;
      else
      KUBERNETES_VERSION=`echo ${KUBERNETES_VERSION} | cut -d"." -f1,2,3`;
      fi
      CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kube-proxy:v${KUBERNETES_VERSION}"
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
  fi
done

# shellcheck disable=SC2010
ls -ltr /dev/* | grep sgx >>  ${VHD_LOGS_FILEPATH} 

df -h

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
} >> ${VHD_LOGS_FILEPATH}

installAscBaseline
