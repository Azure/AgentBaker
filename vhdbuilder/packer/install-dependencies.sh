#!/bin/bash
source /home/packer/provision_installs.sh
source /home/packer/provision_source.sh
source /home/packer/tool_installs.sh
source /home/packer/packer_source.sh
source /home/packer/image_cache.sh

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete

echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

copyPackerFiles

echo ""
echo "Components downloaded in this VHD build (some of the below components might get deleted during cluster provisioning if they are not needed):" >> ${VHD_LOGS_FILEPATH}

AUDITD_ENABLED=true
installDeps
cat << EOF >> ${VHD_LOGS_FILEPATH}
  - apache2-utils
  - apt-transport-https
  - auditd
  - blobfuse=1.1.1
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

installBpftrace
echo "  - bpftrace" >> ${VHD_LOGS_FILEPATH}

MOBY_VERSION="19.03.12"
installMoby
echo "  - moby v${MOBY_VERSION}" >> ${VHD_LOGS_FILEPATH}
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

VNET_CNI_VERSIONS="
1.1.6
1.1.3
1.0.33
"
for VNET_CNI_VERSION in $VNET_CNI_VERSIONS; do
    VNET_CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/azure-cni/v${VNET_CNI_VERSION}/binaries/azure-vnet-cni-linux-amd64-v${VNET_CNI_VERSION}.tgz"
    downloadAzureCNI
    echo "  - Azure CNI version ${VNET_CNI_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

CNI_PLUGIN_VERSIONS="
0.7.6
0.7.5
0.7.1
"
for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

CNI_PLUGIN_VERSIONS="
0.8.6
"
for CNI_PLUGIN_VERSION in $CNI_PLUGIN_VERSIONS; do
    CNI_PLUGINS_URL="https://acs-mirror.azureedge.net/cni-plugins/v${CNI_PLUGIN_VERSION}/binaries/cni-plugins-linux-amd64-v${CNI_PLUGIN_VERSION}.tgz"
    downloadCNI
    echo "  - CNI plugin version ${CNI_PLUGIN_VERSION}" >> ${VHD_LOGS_FILEPATH}
done

#installImg
#echo "  - img" >> ${VHD_LOGS_FILEPATH}
#TODO: sync up with jpalma and replace with buildkit/buildx daemon
if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
  # for UBUNTU RELEASE 18.04 we also want to pre-bake in crictl and system images
  CRICTL_VERSIONS="v1.17.0"
  for CRICTL_VERSION in $CRICTL_VERSIONS; do
      export CRICTL_DOWNLOAD_URL="https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSIONS}/crictl-${CRICTL_VERSIONS}-linux-amd64.tar.gz"
      
      echo "  - crictl version ${CRICTL_VERSION}" >> ${VHD_LOGS_FILEPATH}
  done
fi

# pre-pull system images via docker
pullSystemImages "docker"
# also pre-pull system images via containerd for ubuntu18.04 VHDs
if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
  echo "Pre-pull system images for containerd" >> ${VHD_LOGS_FILEPATH}

  containerd &>/dev/null &
  containerdPID=$1
  echo "Started a containerd process. PID=${containerdPID}" >> ${VHD_LOGS_FILEPATH}
  retrycmd_if_failure 60 1 1200 ctr namespace create k8s.io || exit $ERR_CTR_OPERATION_ERROR
  
  pullSystemImages "containerd"
  
  echo "Killing background containerd process. PID=${pid}" >> ${VHD_LOGS_FILEPATH}
  kill -9 ${containerdPID}
fi

# kubelet and kubectl
# need to cover previously supported version for VMAS scale up scenario
# So keeping as many versions as we can - those unsupported version can be removed when we don't have enough space
# below are the required to support versions
# v1.16.13-hotfix.20200824.1
# v1.16.15-hotfix.20200903
# v1.17.9-hotfix.20200824.1
# v1.17.11-hotfix.20200901
# v1.18.6-hotfix.20200723.1
# v1.18.8
# NOTE that we only keep the latest one per k8s patch version as kubelet/kubectl is decided by VHD version
K8S_VERSIONS="
1.15.7-hotfix.20200326
1.15.10-hotfix.20200408.1
1.15.11-hotfix.20200824.1
1.15.12-hotfix.20200824.1
1.16.9-hotfix.20200529.1
1.16.10-hotfix.20200824.1
1.16.13-hotfix.20200824.1
1.16.15-hotfix.20200903
1.17.3-hotfix.20200601.1
1.17.7-hotfix.20200817.1
1.17.9-hotfix.20200824.1
1.17.11-hotfix.20200901
1.18.2-hotfix.20200624.1
1.18.4-hotfix.20200626.1
1.18.6-hotfix.20200723.1
1.18.8
1.19.0
"
for PATCHED_KUBERNETES_VERSION in ${K8S_VERSIONS}; do
  if (($(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"." -f2) < 17)); then
    HYPERKUBE_URL="mcr.microsoft.com/oss/kubernetes/hyperkube:v${PATCHED_KUBERNETES_VERSION}"
    # NOTE: the KUBERNETES_VERSION will be used to tag the extracted kubelet/kubectl in /usr/local/bin
    # it should match the KUBERNETES_VERSION format(just version number, e.g. 1.15.7, no prefix v)
    # in installKubeletAndKubectl() executed by cse, otherwise cse will need to download the kubelet/kubectl again
    KUBERNETES_VERSION=$(echo ${PATCHED_KUBERNETES_VERSION} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
    # extractHyperkube will extract the kubelet/kubectl binary from the image: ${HYPERKUBE_URL}
    # and put them to /usr/local/bin/kubelet-${KUBERNETES_VERSION}
    extractHyperkube "docker"
    # remove hyperkube here as the one that we really need is pulled later
    docker image rm $HYPERKUBE_URL
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
# v1.16.13-hotfix.20200824.1
# v1.16.15-hotfix.20200903
# v1.17.9-hotfix.20200824.1
# v1.17.11-hotfix.20200901
# v1.18.6-hotfix.20200723.1
# v1.18.8
# NOTE that we keep multiple files per k8s patch version as kubeproxy version is decided by CCP
PATCHED_HYPERKUBE_IMAGES="
1.15.11-hotfix.20200529.1
1.15.12-hotfix.20200623.1
1.15.12-hotfix.20200714.1
1.16.9-hotfix.20200529.1
1.16.10-hotfix.20200623.1
1.16.10-hotfix.20200714.1
1.16.10-hotfix.20200817.1
1.16.10-hotfix.20200824.1
1.16.13-hotfix.20200714.1
1.16.13-hotfix.20200817.1
1.16.13-hotfix.20200824.1
1.16.15-hotfix.20200903
1.17.3-hotfix.20200601.1
1.17.7-hotfix.20200624
1.17.7-hotfix.20200714.1
1.17.7-hotfix.20200714.2
1.17.9-hotfix.20200714.1
1.17.9-hotfix.20200817.1
1.17.9-hotfix.20200824.1
1.17.11-hotfix.20200901
1.18.4-hotfix.20200624
1.18.4-hotfix.20200626.1
1.18.6-hotfix.20200723.1
1.18.6
1.18.8
1.19.0
"
for KUBERNETES_VERSION in ${PATCHED_HYPERKUBE_IMAGES}; do
  # TODO: after CCP chart is done, change below to get hyperkube only for versions less than 1.17 only
  if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) < 19)); then
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/hyperkube:v${KUBERNETES_VERSION}"
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    docker run --rm --entrypoint "" ${CONTAINER_IMAGE}  /bin/sh -c "iptables --version" | grep -v nf_tables && echo "Hyperkube contains no nf_tables"
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
    pullContainerImage "docker" ${CONTAINER_IMAGE}
    docker run --rm --entrypoint "" ${CONTAINER_IMAGE}  /bin/sh -c "iptables --version" | grep -v nf_tables && echo "kube-proxy contains no nf_tables"
    # shellcheck disable=SC2181
    if [[ $? != 0 ]]; then
      echo "Hyperkube contains nf_tables, exiting..."
      exit 99
    fi
    echo "  - ${CONTAINER_IMAGE}" >>${VHD_LOGS_FILEPATH}
  fi
done

#pre-pull add-on images for docker
pullAddonImages "docker"
#pre-pull add-on images for containerd for ubuntu 18.04
if [[ ${UBUNTU_RELEASE} == "18.04" ]]; then
  echo "Pre-pull addon images for containerd" >> ${VHD_LOGS_FILEPATH}

  containerd &>/dev/null &
  containerdPID=$!
  echo "Started a containerd process. PID=${containerdPID}" >> ${VHD_LOGS_FILEPATH}

  pullAddonImages "containerd"
  
  echo "Killing background containerd process. PID=${containerdPID}" >> ${VHD_LOGS_FILEPATH}
  kill -9 ${containerdPID}
fi

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
} >> ${VHD_LOGS_FILEPATH}
