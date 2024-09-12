#!/bin/bash
# ERR_SYSTEMCTL_ENABLE_FAIL=3 Service could not be enabled by systemctl -- DEPRECATED
ERR_SYSTEMCTL_START_FAIL=4 # Service could not be started or enabled by systemctl
ERR_CLOUD_INIT_TIMEOUT=5 # Timeout waiting for cloud-init runcmd to complete
ERR_FILE_WATCH_TIMEOUT=6 # Timeout waiting for a file
ERR_HOLD_WALINUXAGENT=7 # Unable to place walinuxagent apt package on hold during install
ERR_RELEASE_HOLD_WALINUXAGENT=8 # Unable to release hold on walinuxagent apt package after install
ERR_APT_INSTALL_TIMEOUT=9 # Timeout installing required apt packages
ERR_DOCKER_INSTALL_TIMEOUT=20 # Timeout waiting for docker install
ERR_DOCKER_DOWNLOAD_TIMEOUT=21 # Timout waiting for docker downloads
ERR_DOCKER_KEY_DOWNLOAD_TIMEOUT=22 # Timeout waiting to download docker repo key
ERR_DOCKER_APT_KEY_TIMEOUT=23 # Timeout waiting for docker apt-key
ERR_DOCKER_START_FAIL=24 # Docker could not be started by systemctl
ERR_MOBY_APT_LIST_TIMEOUT=25 # Timeout waiting for moby apt sources
ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT=26 # Timeout waiting for MS GPG key download
ERR_MOBY_INSTALL_TIMEOUT=27 # Timeout waiting for moby-docker install
ERR_CONTAINERD_INSTALL_TIMEOUT=28 # Timeout waiting for moby-containerd install
ERR_RUNC_INSTALL_TIMEOUT=29 # Timeout waiting for moby-runc install
ERR_K8S_RUNNING_TIMEOUT=30 # Timeout waiting for k8s cluster to be healthy
ERR_K8S_DOWNLOAD_TIMEOUT=31 # Timeout waiting for Kubernetes downloads
ERR_KUBECTL_NOT_FOUND=32 # kubectl client binary not found on local disk
ERR_IMG_DOWNLOAD_TIMEOUT=33 # Timeout waiting for img download
ERR_KUBELET_START_FAIL=34 # kubelet could not be started by systemctl
ERR_DOCKER_IMG_PULL_TIMEOUT=35 # Timeout trying to pull a Docker image
ERR_CONTAINERD_CTR_IMG_PULL_TIMEOUT=36 # Timeout trying to pull a containerd image via cli tool ctr
ERR_CONTAINERD_CRICTL_IMG_PULL_TIMEOUT=37 # Timeout trying to pull a containerd image via cli tool crictl
ERR_CONTAINERD_INSTALL_FILE_NOT_FOUND=38 # Unable to locate containerd debian pkg file
ERR_CONTAINERD_VERSION_INVALID=39 # Containerd version is invalid
ERR_CNI_DOWNLOAD_TIMEOUT=41 # Timeout waiting for CNI downloads
ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT=42 # Timeout waiting for https://packages.microsoft.com/config/ubuntu/16.04/packages-microsoft-prod.deb
ERR_MS_PROD_DEB_PKG_ADD_FAIL=43 # Failed to add repo pkg file
# ERR_FLEXVOLUME_DOWNLOAD_TIMEOUT=44 Failed to add repo pkg file -- DEPRECATED
ERR_ORAS_DOWNLOAD_ERROR=45 # Unable to install oras
ERR_SYSTEMD_INSTALL_FAIL=48 # Unable to install required systemd version
ERR_MODPROBE_FAIL=49 # Unable to load a kernel module using modprobe
ERR_OUTBOUND_CONN_FAIL=50 # Unable to establish outbound connection
ERR_K8S_API_SERVER_CONN_FAIL=51 # Unable to establish connection to k8s api serve
ERR_K8S_API_SERVER_DNS_LOOKUP_FAIL=52 # Unable to resolve k8s api server name
ERR_K8S_API_SERVER_AZURE_DNS_LOOKUP_FAIL=53 # Unable to resolve k8s api server name due to Azure DNS issue
ERR_KATA_KEY_DOWNLOAD_TIMEOUT=60 # Timeout waiting to download kata repo key
ERR_KATA_APT_KEY_TIMEOUT=61 # Timeout waiting for kata apt-key
ERR_KATA_INSTALL_TIMEOUT=62 # Timeout waiting for kata install
ERR_VHD_FILE_NOT_FOUND=65 # VHD log file not found on VM built from VHD distro (previously classified as exit code 124)
ERR_CONTAINERD_DOWNLOAD_TIMEOUT=70 # Timeout waiting for containerd downloads
ERR_RUNC_DOWNLOAD_TIMEOUT=71 # Timeout waiting for runc downloads
ERR_CUSTOM_SEARCH_DOMAINS_FAIL=80 # Unable to configure custom search domains
ERR_GPU_DOWNLOAD_TIMEOUT=83 # Timeout waiting for GPU driver download
ERR_GPU_DRIVERS_START_FAIL=84 # nvidia-modprobe could not be started by systemctl
ERR_GPU_DRIVERS_INSTALL_TIMEOUT=85 # Timeout waiting for GPU drivers install
ERR_GPU_DEVICE_PLUGIN_START_FAIL=86 # nvidia device plugin could not be started by systemctl
ERR_GPU_INFO_ROM_CORRUPTED=87 # info ROM corrupted error when executing nvidia-smi
ERR_SGX_DRIVERS_INSTALL_TIMEOUT=90 # Timeout waiting for SGX prereqs to download
ERR_SGX_DRIVERS_START_FAIL=91 # Failed to execute SGX driver binary
ERR_APT_DAILY_TIMEOUT=98 # Timeout waiting for apt daily updates
ERR_APT_UPDATE_TIMEOUT=99 # Timeout waiting for apt-get update to complete
ERR_CSE_PROVISION_SCRIPT_NOT_READY_TIMEOUT=100 # Timeout waiting for cloud-init to place this script on the vm
ERR_APT_DIST_UPGRADE_TIMEOUT=101 # Timeout waiting for apt-get dist-upgrade to complete
ERR_APT_PURGE_FAIL=102 # Error purging distro packages
ERR_SYSCTL_RELOAD=103 # Error reloading sysctl config
ERR_CIS_ASSIGN_ROOT_PW=111 # Error assigning root password in CIS enforcement
ERR_CIS_ASSIGN_FILE_PERMISSION=112 # Error assigning permission to a file in CIS enforcement
ERR_PACKER_COPY_FILE=113 # Error writing a file to disk during VHD CI
ERR_CIS_APPLY_PASSWORD_CONFIG=115 # Error applying CIS-recommended passwd configuration
ERR_SYSTEMD_DOCKER_STOP_FAIL=116 # Error stopping dockerd
ERR_CRICTL_DOWNLOAD_TIMEOUT=117 # Timeout waiting for crictl downloads
ERR_CRICTL_OPERATION_ERROR=118 # Error executing a crictl operation
ERR_CTR_OPERATION_ERROR=119 # Error executing a ctr containerd cli operation

# Azure Stack specific errors
ERR_AZURE_STACK_GET_ARM_TOKEN=120 # Error generating a token to use with Azure Resource Manager
ERR_AZURE_STACK_GET_NETWORK_CONFIGURATION=121 # Error fetching the network configuration for the node
ERR_AZURE_STACK_GET_SUBNET_PREFIX=122 # Error fetching the subnet address prefix for a subnet ID

# Error code 124 is returned when a `timeout` command times out, and --preserve-status is not specified: https://man7.org/linux/man-pages/man1/timeout.1.html
ERR_VHD_BUILD_ERROR=125 # Reserved for VHD CI exit conditions

ERR_SWAP_CREATE_FAIL=130 # Error allocating swap file
ERR_SWAP_CREATE_INSUFFICIENT_DISK_SPACE=131 # Error insufficient disk space for swap file creation

ERR_TELEPORTD_DOWNLOAD_ERR=150 # Error downloading teleportd binary
ERR_TELEPORTD_INSTALL_ERR=151 # Error installing teleportd binary
ERR_ARTIFACT_STREAMING_DOWNLOAD=152 # Error downloading mirror proxy and overlaybd components
ERR_ARTIFACT_STREAMING_INSTALL=153 # Error installing mirror proxy and overlaybd components

ERR_HTTP_PROXY_CA_CONVERT=160 # Error converting http proxy ca cert from pem to crt format
ERR_UPDATE_CA_CERTS=161 # Error updating ca certs to include user-provided certificates
ERR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_TIMEOUT=169 # Timeout waiting for secure TLS bootrstrap kubelet exec plugin download

ERR_DISBALE_IPTABLES=170 # Error disabling iptables service

ERR_KRUSTLET_DOWNLOAD_TIMEOUT=171 # Timeout waiting for krustlet downloads
ERR_DISABLE_SSH=172 # Error disabling ssh service
ERR_PRIMARY_NIC_IP_NOT_FOUND=173 # Error fetching primary NIC IP address
ERR_INSERT_IMDS_RESTRICTION_RULE_INTO_MANGLE_TABLE=174 # Error insert imds restriction rule into mangle table
ERR_INSERT_IMDS_RESTRICTION_RULE_INTO_FILTER_TABLE=175 # Error insert imds restriction rule into filter table
ERR_DELETE_IMDS_RESTRICTION_RULE_FROM_MANGLE_TABLE=176 # Error delete imds restriction rule from mangle table
ERR_DELETE_IMDS_RESTRICTION_RULE_FROM_FILTER_TABLE=177 # Error delete imds restriction rule from filter table

ERR_VHD_REBOOT_REQUIRED=200 # Reserved for VHD reboot required exit condition
ERR_NO_PACKAGES_FOUND=201 # Reserved for no security packages found exit condition
ERR_SNAPSHOT_UPDATE_START_FAIL=202 # snapshot-update could not be started by systemctl

ERR_PRIVATE_K8S_PKG_ERR=203 # Error downloading (at build-time) or extracting (at run-time) private kubernetes packages
ERR_K8S_INSTALL_ERR=204 # Error installing or setting up kubernetes binaries on disk

ERR_SYSTEMCTL_MASK_FAIL=2 # Service could not be masked by systemctl

ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT=205 # Timeout waiting for credential provider downloads

ERR_CNI_VERSION_INVALID=206 # reference CNI (not azure cni) needs a valid version in components.json

# For both Ubuntu and Mariner/AzureLinux, /etc/*-release should exist.
# In AzureLinux 3.0, /etc/*-release are symlinks to /usr/lib/*-release, so the find command includes -type f,l.

ERR_ORAS_PULL_K8S_FAIL=207 # Error pulling kube-node artifact via oras from registry
ERR_ORAS_PULL_FAIL_RESERVE_1=208 # Error pulling artifact with oras from registry
ERR_ORAS_PULL_CONTAINERD_WASM=209 # Error pulling containerd wasm artifact with oras from registry
ERR_ORAS_PULL_FAIL_RESERVE_3=210 # Error pulling artifact with oras from registry
ERR_ORAS_PULL_FAIL_RESERVE_4=211 # Error pulling artifact with oras from registry
ERR_ORAS_PULL_FAIL_RESERVE_5=212 # Error pulling artifact with oras from registry

# Error checking nodepools tags for whether we need to disable kubelet serving certificate rotation
ERR_LOOKUP_DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION_TAG=213

# For both Ubuntu and Mariner, /etc/*-release should exist.
# For unit tests, the OS and OS_VERSION will be set in the unit test script.
# So whether it's if or else actually doesn't matter to our unit test.
if find /etc -type f,l -name "*-release" -print -quit 2>/dev/null | grep -q '.'; then
    OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
    OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')
else
# This is only for unit test purpose. For example, a Mac OS dev box doesn't have /etc/*-release, then the unit test will continue.
    echo "/etc/*-release not found"
fi

UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
MARINER_KATA_OS_NAME="MARINERKATA"
AZURELINUX_OS_NAME="AZURELINUX"
KUBECTL=/usr/local/bin/kubectl
DOCKER=/usr/bin/docker
# this will be empty during VHD build
# but vhd build runs with `set -o nounset`
# so needs a default value
# prefer empty string to avoid potential "it works but did something weird" scenarios
export GPU_DV="${GPU_DRIVER_VERSION:=}"
export GPU_DEST=/usr/local/nvidia
NVIDIA_DOCKER_VERSION=2.8.0-1
DOCKER_VERSION=1.13.1-1
NVIDIA_CONTAINER_RUNTIME_VERSION="3.6.0"
export NVIDIA_DRIVER_IMAGE_SHA="${GPU_IMAGE_SHA:=}"
export NVIDIA_DRIVER_IMAGE_TAG="${GPU_DV}-${NVIDIA_DRIVER_IMAGE_SHA}"
export NVIDIA_DRIVER_IMAGE="mcr.microsoft.com/aks/aks-gpu"
export CTR_GPU_INSTALL_CMD="ctr run --privileged --rm --net-host --with-ns pid:/proc/1/ns/pid --mount type=bind,src=/opt/gpu,dst=/mnt/gpu,options=rbind --mount type=bind,src=/opt/actions,dst=/mnt/actions,options=rbind"
export DOCKER_GPU_INSTALL_CMD="docker run --privileged --net=host --pid=host -v /opt/gpu:/mnt/gpu -v /opt/actions:/mnt/actions --rm"
APT_CACHE_DIR=/var/cache/apt/archives/
PERMANENT_CACHE_DIR=/root/aptcache/
EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
CURL_OUTPUT=/tmp/curl_verbose.out
ORAS_OUTPUT=/tmp/oras_verbose.out
ORAS_REGISTRY_CONFIG_FILE=/etc/oras/config.yaml # oras registry auth config file, not used, but have to define to avoid error "Error: failed to get user home directory: $HOME is not defined" 

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout "${@}" && break || \
        if [ $i -eq $retries ]; then
            echo Executed \"$@\" $i times;
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed \"$@\" $i times;
}
retrycmd_nslookup() {
    wait_sleep=$1; timeout=$2; total_timeout=$3; record=$4
    start_time=$(date +%s)
    while true; do
        nslookup -timeout=$timeout -retry=0 $record && break || \
        current_time=$(date +%s)
        # Check if the total_timeout has been reached
        if [ $((current_time - start_time)) -ge $total_timeout ]; then
            echo "Total timeout $total_timeout reached, nslookup -timeout=$timeout -retry=0 $record failed"
            return 1
        fi
        sleep $wait_sleep
    done
    current_time=$(date +%s)
    echo "Executed nslookup -timeout=$timeout -retry=0 $record for $((current_time - start_time)) seconds";
}
retrycmd_if_failure_no_stats() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
retrycmd_get_tarball() {
    tar_retries=$1; wait_sleep=$2; tarball=$3; url=$4
    echo "${tar_retries} retries"
    for i in $(seq 1 $tar_retries); do
        tar -tzf $tarball && break || \
        if [ $i -eq $tar_retries ]; then
            return 1
        else
            timeout 60 curl -fsSLv $url -o $tarball > $CURL_OUTPUT 2>&1
            if [[ $? != 0 ]]; then
                cat $CURL_OUTPUT
            fi
            sleep $wait_sleep
        fi
    done
}
retrycmd_get_tarball_from_registry_with_oras() {
    tar_retries=$1; wait_sleep=$2; tarball=$3; url=$4
    tar_folder=$(dirname "$tarball")
    echo "${tar_retries} retries"
    for i in $(seq 1 $tar_retries); do
        tar -tzf $tarball && break || \
        if [ $i -eq $tar_retries ]; then
            return 1
        else
            # TODO: support private acr via kubelet identity
            timeout 60 oras pull $url -o $tar_folder --registry-config ${ORAS_REGISTRY_CONFIG_FILE} > $ORAS_OUTPUT 2>&1
            if [[ $? != 0 ]]; then
                cat $ORAS_OUTPUT
            fi
            sleep $wait_sleep
        fi
    done
}
retrycmd_curl_file() {
    curl_retries=$1; wait_sleep=$2; timeout=$3; filepath=$4; url=$5
    echo "${curl_retries} retries"
    for i in $(seq 1 $curl_retries); do
        [[ -f $filepath ]] && break
        if [ $i -eq $curl_retries ]; then
            return 1
        else
            timeout $timeout curl -fsSLv $url -o $filepath 2>&1 | tee $CURL_OUTPUT >/dev/null
            if [[ $? != 0 ]]; then
                cat $CURL_OUTPUT
            fi
            sleep $wait_sleep
        fi
    done
}
wait_for_file() {
    retries=$1; wait_sleep=$2; filepath=$3
    paved=/opt/azure/cloud-init-files.paved
    grep -Fq "${filepath}" $paved && return 0
    for i in $(seq 1 $retries); do
        grep -Fq '#EOF' $filepath && break
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    sed -i "/#EOF/d" $filepath
    echo $filepath >> $paved
}
systemctl_restart() {
    retries=$1; wait_sleep=$2; timeout=$3 svcname=$4
    for i in $(seq 1 $retries); do
        timeout $timeout systemctl daemon-reload
        timeout $timeout systemctl restart $svcname && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            systemctl status $svcname --no-pager -l
            journalctl -u $svcname
            sleep $wait_sleep
        fi
    done
}
systemctl_stop() {
    retries=$1; wait_sleep=$2; timeout=$3 svcname=$4
    for i in $(seq 1 $retries); do
        timeout $timeout systemctl daemon-reload
        timeout $timeout systemctl stop $svcname && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
systemctl_disable() {
    retries=$1; wait_sleep=$2; timeout=$3 svcname=$4
    for i in $(seq 1 $retries); do
        timeout $timeout systemctl daemon-reload
        timeout $timeout systemctl disable $svcname && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
sysctl_reload() {
    retries=$1; wait_sleep=$2; timeout=$3
    for i in $(seq 1 $retries); do
        timeout $timeout sysctl --system && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
}
version_gte() {
  test "$(printf '%s\n' "$@" | sort -rV | head -n 1)" == "$1"
}

systemctlEnableAndStart() {
    systemctl_restart 100 5 30 $1
    RESTART_STATUS=$?
    systemctl status $1 --no-pager -l > /var/log/azure/$1-status.log
    if [ $RESTART_STATUS -ne 0 ]; then
        echo "$1 could not be started"
        return 1
    fi
    if ! retrycmd_if_failure 120 5 25 systemctl enable $1; then
        echo "$1 could not be enabled by systemctl"
        return 1
    fi
}

systemctlDisableAndStop() {
    if systemctl list-units --full --all | grep -q "$1.service"; then
        systemctl_stop 20 5 25 $1 || echo "$1 could not be stopped"
        systemctl_disable 20 5 25 $1 || echo "$1 could not be disabled"
    fi
}

# return true if a >= b
semverCompare() {
    VERSION_A=$(echo $1 | cut -d "+" -f 1)
    VERSION_B=$(echo $2 | cut -d "+" -f 1)
    [[ "${VERSION_A}" == "${VERSION_B}" ]] && return 0
    sorted=$(echo ${VERSION_A} ${VERSION_B} | tr ' ' '\n' | sort -V )
    highestVersion=$(IFS= echo "${sorted}" | cut -d$'\n' -f2)
    [[ "${VERSION_A}" == ${highestVersion} ]] && return 0
    return 1
}
downloadDebPkgToFile() {
    PKG_NAME=$1
    PKG_VERSION=$2
    PKG_DIRECTORY=$3
    mkdir -p $PKG_DIRECTORY
    # shellcheck disable=SC2164
    pushd ${PKG_DIRECTORY}
    retrycmd_if_failure 10 5 600 apt-get download ${PKG_NAME}=${PKG_VERSION}*
    # shellcheck disable=SC2164
    popd
}
apt_get_download() {
  retries=$1; wait_sleep=$2; shift && shift;
  local ret=0
  pushd $APT_CACHE_DIR || return 1
  for i in $(seq 1 $retries); do
    dpkg --configure -a --force-confdef
    wait_for_apt_locks
    apt-get -o Dpkg::Options::=--force-confold download -y "${@}" && break
    if [ $i -eq $retries ]; then ret=1; else sleep $wait_sleep; fi
  done
  popd || return 1
  return $ret
}
getCPUArch() {
    arch=$(uname -m)
    if [[ ${arch,,} == "aarch64" || ${arch,,} == "arm64"  ]]; then
        echo "arm64"
    else
        echo "amd64"
    fi
}
isARM64() {
    if [[ $(getCPUArch) == "arm64" ]]; then
        echo 1
    else
        echo 0
    fi
}

isRegistryUrl() {
    local binary_url=$1
    registry_regex='^.+\/.+\/.+:.+$'
    if [[ ${binary_url} =~ $registry_regex ]]; then # check if the binary_url is in the format of mcr.microsoft.com/componant/binary:1.0"
        return 0 # true
    fi
    return 1 # false
}

logs_to_events() {
    # local vars here allow for nested function tracking
    # installContainerRuntime for example
    local task=$1; shift
    local eventsFileName=$(date +%s%3N)

    local startTime=$(date +"%F %T.%3N")
    ${@}
    ret=$?
    local endTime=$(date +"%F %T.%3N")

    # arg names are defined by GA and all these are required to be correctly read by GA
    # EventPid, EventTid are required to be int. No use case for them at this point.
    json_string=$( jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "Completed: $*" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )
    echo ${json_string} > ${EVENTS_LOGGING_DIR}${eventsFileName}.json

    # this allows an error from the command at ${@} to be returned and correct code assigned in cse_main
    if [ "$ret" != "0" ]; then
      return $ret
    fi
}

should_skip_nvidia_drivers() {
    set -x
    body=$(curl -fsSL -H "Metadata: true" --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2021-02-01")
    ret=$?
    if [ "$ret" != "0" ]; then
      return $ret
    fi
    should_skip=$(echo "$body" | jq -e '.compute.tagsList | map(select(.name | test("SkipGpuDriverInstall"; "i")))[0].value // "false" | test("true"; "i")')
    echo "$should_skip"
}

should_disable_kubelet_serving_certificate_rotation() {
    set -x
    body=$(curl -fsSL -H "Metadata: true" --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2021-02-01")
    ret=$?
    if [ "$ret" != "0" ]; then
      return $ret
    fi
    should_disable=$(echo "$body" | jq -r '.compute.tagsList[] | select(.name == "aks-disable-kubelet-serving-certificate-rotation") | .value')
    echo "$should_disable"
}

isMarinerOrAzureLinux() {
    local os=$1
    if [[ $os == $MARINER_OS_NAME ]] || [[ $os == $MARINER_KATA_OS_NAME ]] || [[ $os == $AZURELINUX_OS_NAME ]]; then
        return 0
    fi
    return 1
}

installJq() {
  # jq is not available until downloaded in install-dependencies.sh with the installDeps function
  # but it is needed earlier to call the capture_benchmarks function in pre-install-dependencies.sh
  output=$(jq --version)
  if [ -n "$output" ]; then
    echo "$output"
  else
    if isMarinerOrAzureLinux "$OS"; then
      sudo tdnf install -y jq && echo "jq was installed: $(jq --version)"
    else
      apt_get_install 5 1 60 jq && echo "jq was installed: $(jq --version)"
    fi
  fi
}

check_array_size() {
  declare -n array_name=$1
  local array_size=${#array_name[@]}
  if [[ ${array_size} -gt 0 ]]; then
    last_index=$(( ${#array_name[@]} - 1 ))
  else
    return 1
  fi
}

capture_benchmark() {
  set +x
  local title="$1"
  title="${title//[[:space:]]/_}"
  title="${title//-/_}"
  local is_final_section=${2:-false}

  local current_time=$(date +%s)
  if [[ "$is_final_section" == true ]]; then
    local start_time=$script_start_stopwatch
  else
    local start_time=$section_start_stopwatch
  fi
  
  total_time_elapsed=$(date -d@$((current_time - start_time)) -u +%H:%M:%S)
  benchmarks[$title]=${total_time_elapsed}
  benchmarks_order+=($title) # use this array to maintain order of benchmarks

  # reset timers for next section
  section_start_stopwatch=$(date +%s)
}

process_benchmarks() {
  set +x
  check_array_size benchmarks || { echo "Benchmarks array is empty"; return; }
  # create script object, then append each section object to it in the for loop
  script_object=$(jq -n --arg script_name "${SCRIPT_NAME}" '{($script_name): {}}')

  for ((i=0; i<${#benchmarks_order[@]}; i+=1)); do
    section_name=${benchmarks_order[i]}
    section_object=$(jq -n --arg section_name "${section_name}" --arg total_time_elapsed "${benchmarks[${section_name}]}" \
    '{($section_name): $total_time_elapsed'})
    script_object=$(jq -n --argjson script_object "$script_object" --argjson section_object "$section_object" --arg script_name "${SCRIPT_NAME}" \
    '$script_object | .[$script_name] += $section_object')
  done
 
  jq ". += $script_object" ${VHD_BUILD_PERF_DATA} > temp-build-perf-file.json && mv temp-build-perf-file.json ${VHD_BUILD_PERF_DATA}
  chmod 755 ${VHD_BUILD_PERF_DATA}
}

#return proper release metadata for the package based on the os and osVersion
#e.g., For os UBUNTU 18.04, if there is a release "r1804" defined in components.json, then set RELEASE to "r1804"
#Otherwise set RELEASE to "current"
returnRelease() {
    local package="$1"
    local os="$2"
    local osVersion="$3"
    RELEASE="current"
    local osVersionWithoutDot=$(echo "${osVersion}" | sed 's/\.//g')
    #For UBUNTU, if $osVersion is 18.04 and "r1804" is also defined in components.json, then $release is set to "r1804"
    #Similarly for 20.04 and 22.04. Otherwise $release is set to .current.
    #For MARINER/AZURELINUX, the release is always set to "current" now.
    if isMarinerOrAzureLinux "${os}"; then
        return 0
    fi
    if [[ $(echo "${package}" | jq ".downloadURIs.ubuntu.\"r${osVersionWithoutDot}\"") != "null" ]]; then
        RELEASE="\"r${osVersionWithoutDot}\""
    fi
}

returnPackageVersions() {
    local package="$1"
    local os="$2"
    local osVersion="$3"
    RELEASE="current"
    returnRelease "${package}" "${os}" "${osVersion}"
    local osLowerCase=$(echo "${os}" | tr '[:upper:]' '[:lower:]')
    PACKAGE_VERSIONS=()

    #if .downloadURIs.${osLowerCase} exist, then get the versions from there.
    #otherwise get the versions from .downloadURIs.default 
    if [[ $(echo "${package}" | jq ".downloadURIs.${osLowerCase}") != "null" ]]; then
        # Check if there are any versions available for the specific OS
        if jq -e ".downloadURIs.${osLowerCase}.${RELEASE}.versions | length == 0" <<< "${package}" > /dev/null; then
            return
        fi
        versions=$(echo "${package}" | jq ".downloadURIs.${osLowerCase}.${RELEASE}.versions[]" -r)
        for version in ${versions[@]}; do
            PACKAGE_VERSIONS+=("${version}")
        done
        return
    fi
    versions=$(echo "${package}" | jq ".downloadURIs.default.${RELEASE}.versions[]" -r)
    for version in ${versions[@]}; do
        PACKAGE_VERSIONS+=("${version}")
    done
    return 0
}

returnPackageDownloadURL() {
    local package=$1
    local os=$2
    local osVersion=$3
    RELEASE="current"
    returnRelease "${package}" "${os}" "${osVersion}"
    local osLowerCase=$(echo "${os}" | tr '[:upper:]' '[:lower:]')
    
    #if .downloadURIs.${osLowerCase} exist, then get the downloadURL from there.
    #otherwise get the downloadURL from .downloadURIs.default 
    if [[ $(echo "${package}" | jq ".downloadURIs.${osLowerCase}") != "null" ]]; then
        downloadURL=$(echo "${package}" | jq ".downloadURIs.${osLowerCase}.${RELEASE}.downloadURL" -r)
        [ "${downloadURL}" = "null" ] && PACKAGE_DOWNLOAD_URL="" || PACKAGE_DOWNLOAD_URL="${downloadURL}"
        return
    fi
    downloadURL=$(echo "${package}" | jq ".downloadURIs.default.${RELEASE}.downloadURL" -r)
    [ "${downloadURL}" = "null" ] && PACKAGE_DOWNLOAD_URL="" || PACKAGE_DOWNLOAD_URL="${downloadURL}"
    return    
}

#HELPERSEOF
