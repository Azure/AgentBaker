#!/bin/bash

#time_stamps is an associative array that links the logical steps of the install script with timestamps
#Logicalorder is a regular array that remembers the actual order of the steps (associative arrays are not indexed)
declare -A time_stamps=()   
declare -a logical_order=() 

#The record_benchmark function adds the current step into logical_order and then associates that step with a start and end time in the timestamps array
record_benchmark () { dt_stamp=$(date +%H:%M:%S); logical_order+=( "${1}" ); time_stamps["${1}"]=$dt_stamp; }

#The PrintBenchmarks function uses the logical_order array as a control to chronologically iterate over the step/timestamp pairs in the timestamps array
print_benchmark_results () { echo; echo; echo "Benchmarking Results:"; echo; for i in "${logical_order[@]}"; do echo "   $i: ${time_stamps[$i]}"; done; echo; echo; }

#Used as a reset for the stopwatch functions
start_time=$(date +%s)

#This function resets the the stopwatch start time
start_watch () {
  start_time=$(date +%s)
}

#This function captures the current time and subtracts the start time of the stopwatch, resulting in the total elapsed time
stop_watch () {
  local current_time=$(date +%s)
  local difference_in_seconds=$((current_time - start_time))

  local elapsed_hours=$(($difference_in_seconds / 3600))
  local remaining_seconds_to_minutes=$(($difference_in_seconds % 3600))

  local elapsed_minutes=$(($remaining_seconds_to_minutes / 60))
  local remaining_seconds_to_seconds=$(($remaining_seconds_to_minutes % 60))

  local elapsed_seconds=$(($remaining_seconds_to_seconds % 60))

  echo
  printf "'${1}' - Total Time Elapsed: %02d:%02d:%02d" $elapsed_hours $elapsed_minutes $elapsed_seconds
  echo
}

#Benchmark 1 Start 
record_benchmark 'Declare variables / remove comments (Lines 44 - 50) Start'
start_watch

OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
OS_VERSION=$(sort -r /etc/*-release | gawk 'match($0, /^(VERSION_ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }' | tr -d '"')
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"

#the following sed removes all comments of the format {{/* */}}
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/provision_source.sh
sed -i 's/{{\/\*[^*]*\*\/}}//g' /home/packer/tool_installs_distro.sh

record_benchmark 'Declare variables / remove comments (Lines 44 - 50) End'
stop_watch 'Declare variables / remove comments'
#Benchmark 1 End
#Benchmark 2 Start
record_benchmark 'Execute /home/packer files (Lines 59 - 65) Start'
start_watch

source /home/packer/provision_installs.sh
source /home/packer/provision_installs_distro.sh
source /home/packer/provision_source.sh
source /home/packer/provision_source_distro.sh
source /home/packer/tool_installs.sh
source /home/packer/tool_installs_distro.sh
source /home/packer/packer_source.sh

record_benchmark 'Execute /home/packer files (Lines 59 - 65) End'
stop_watch 'Execute /home/packer files'
#Benchmark 2 End
#Benchmark 3 Start
record_benchmark 'Create post-build test (Lines 74 - 83) Start'
start_watch

CPU_ARCH=$(getCPUArch)  #amd64 or arm64
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
COMPONENTS_FILEPATH=/opt/azure/components.json
MANIFEST_FILEPATH=/opt/azure/manifest.json
KUBE_PROXY_IMAGES_FILEPATH=/opt/azure/kube-proxy-images.json
#this is used by post build test to check whether the compoenents do indeed exist
cat components.json > ${COMPONENTS_FILEPATH}
cat manifest.json > ${MANIFEST_FILEPATH}
cat ${THIS_DIR}/kube-proxy-images.json > ${KUBE_PROXY_IMAGES_FILEPATH}
echo "Starting build on " $(date) > ${VHD_LOGS_FILEPATH}

record_benchmark 'Create post-build test (Lines 74 - 83) End'
stop_watch 'Create post-build test'
#Benchmark 3 End
#Benchmark 4 Start
record_benchmark 'Set permissions if Mariner (Lines 92 - 96) Start'
start_watch

if [[ $OS == $MARINER_OS_NAME ]]; then
  chmod 755 /opt
  chmod 755 /opt/azure
  chmod 644 ${VHD_LOGS_FILEPATH}
fi

record_benchmark 'Set permissions if Mariner (Lines 92 - 96) End'
stop_watch 'Set permissions if Mariner'
#Benchmark 4 End
#Benchmark 5 Start
record_benchmark 'Copy packer files and start disk queue (Lines 105 - 106) Start'
start_watch

copyPackerFiles
systemctlEnableAndStart disk_queue || exit 1

record_benchmark 'Copy packer files and start disk queue (Lines 105 - 106) End'
stop_watch 'Copy packer files and start disk queue'
#Benchmark 5 End
#Benchmark 6 Start
record_benchmark 'Make certs directory, set permissions, and update certs (Lines 115 - 117) Start'
start_watch

mkdir /opt/certs
chmod 1666 /opt/certs
systemctlEnableAndStart update_certs.path || exit 1

record_benchmark 'Make certs directory, set permissions, and update certs (Lines 115 - 117) End'
stop_watch 'Make certs directory, set permissions, and update certs'
#Benchmark 6 End
#Benchmark 7 Start
record_benchmark 'Start system logs and AKS log collector (Lines 126 - 131) Start'
start_watch

systemctlEnableAndStart ci-syslog-watcher.path || exit 1
systemctlEnableAndStart ci-syslog-watcher.service || exit 1

# enable AKS log collector
echo -e "\n# Disable WALA log collection because AKS Log Collector is installed.\nLogs.Collect=n" >> /etc/waagent.conf || exit 1
systemctlEnableAndStart aks-log-collector.timer || exit 1

record_benchmark 'Start system logs and AKS log collector (Lines 126 - 131) End'
stop_watch 'Start system logs and AKS log collector'
#Benchmark 7 End
#Benchmark 8 Start
record_benchmark 'Start modified log-rotate service and remove auto-generated default log-rotate service (Lines 141 - 142) Start'
start_watch

# enable the modified logrotate service and remove the auto-generated default logrotate cron job if present
systemctlEnableAndStart logrotate.timer || exit 1
rm -f /etc/cron.daily/logrotate

record_benchmark 'Start modified log-rotate service and remove auto-generated default log-rotate service (Lines 141 - 142) End'
stop_watch 'Start modified log-rotate service and remove auto-generated default log-rotate service'
#Benchmark 8 End
#Benchmark 9 Start
record_benchmark 'Sync container logs (Line 151) Start'
start_watch

systemctlEnableAndStart sync-container-logs.service || exit 1

record_benchmark 'Sync container logs (Line 151) End'
stop_watch 'Sync container logs'
#Benchmark 9 End
#Benchmark 10 Start
record_benchmark 'Handle Marine and FIPS Configurations (Lines 161 - 192) Start'
start_watch

# First handle Mariner + FIPS
if [[ ${OS} == ${MARINER_OS_NAME} ]]; then
  dnf_makecache || exit $ERR_APT_UPDATE_TIMEOUT
  dnf_update || exit $ERR_APT_DIST_UPGRADE_TIMEOUT
  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # This is FIPS install for Mariner and has nothing to do with Ubuntu Advantage
    echo "Install FIPS for Mariner SKU"
    installFIPS
  fi
else
  # Enable ESM on Ubuntu
  autoAttachUA

  # Run apt get update to refresh repo list
  # Run apt dist get upgrade to install packages/kernels

  # CVM breaks on kernel image updates due to nullboot package post-install.
  # it relies on boot measurements from real tpm hardware.
  # building on a real CVM would solve this, but packer doesn't support it.
  # we could make upstream changes but that takes time, and we are broken now.
  # so we just hold the kernel image packages for now on CVM.
  # this still allows us base image and package updates on a weekly cadence.
  if [[ "$IMG_SKU" != "20_04-lts-cvm" ]]; then
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_dist_upgrade || exit $ERR_APT_DIST_UPGRADE_TIMEOUT    
  fi

  if [[ "${ENABLE_FIPS,,}" == "true" ]]; then
    # This is FIPS Install for Ubuntu, it purges non FIPS Kernel and attaches UA FIPS Updates
    echo "Install FIPS for Ubuntu SKU"
    installFIPS
  fi
fi

record_benchmark 'Handle Marine and FIPS Configurations (Lines 161 - 192) End'
stop_watch 'Handle Marine and FIPS Configurations'
#Benchmark 10 End
#Benchmark 11 Start
record_benchmark 'Handle Azure Linux + CgroupV2 (Lines 202 - 219) Start'
start_watch

# Handle Azure Linux + CgroupV2
if [[ ${OS} == ${MARINER_OS_NAME} ]] && [[ "${ENABLE_CGROUPV2,,}" == "true" ]]; then
  enableCgroupV2forAzureLinux
fi

if [[ "${UBUNTU_RELEASE}" == "22.04" ]]; then
  echo "Logging the currently running kernel: $(uname -r)"
  echo "Before purging kernel, here is a list of kernels/headers installed:"; dpkg -l 'linux-*azure*'

  # Purge all current kernels and dependencies
  DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y $(dpkg-query -W 'linux-*azure*' | awk '$2 != "" { print $1 }' | paste -s)
  echo "After purging kernel, dpkg list should be empty"; dpkg -l 'linux-*azure*'

  # Install lts-22.04 kernel
  DEBIAN_FRONTEND=noninteractive apt-get install -y linux-image-azure-lts-22.04 linux-cloud-tools-azure-lts-22.04 linux-headers-azure-lts-22.04 linux-modules-extra-azure-lts-22.04 linux-tools-azure-lts-22.04
  echo "After installing new kernel, here is a list of kernels/headers installed"; dpkg -l 'linux-*azure*'
  
  update-grub
fi

record_benchmark 'Handle Azure Linux + CgroupV2 (Lines 202 - 219) End'
stop_watch 'Handle Azure Linux + CgroupV2'
#Benchmark 11 End
#End of Benchmarks
echo
print_benchmark_results
echo

echo "pre-install-dependencies step finished successfully"