#!/bin/bash
git clone https://github.com/Azure/AgentBaker.git 2>/dev/null
source ./AgentBaker/parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh 2>/dev/null
source ./AgentBaker/parts/linux/cloud-init/artifacts/cse_helpers.sh 2>/dev/null
COMPONENTS_FILEPATH=/opt/azure/components.json
KUBE_PROXY_IMAGES_FILEPATH=/opt/azure/kube-proxy-images.json
MANIFEST_FILEPATH=/opt/azure/manifest.json
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"
CONTAINER_RUNTIME="$1"
OS_VERSION="$2"
ENABLE_FIPS="$3"
OS_SKU="$4"

testFilesDownloaded() {
  test="testFilesDownloaded"
  containerRuntime=$1
  if [[ $(isARM64) == 1 ]]; then
    return
  fi
  echo "$test:Start"
  filesToDownload=$(jq .DownloadFiles[] --monochrome-output --compact-output <$COMPONENTS_FILEPATH)

  for fileToDownload in ${filesToDownload[*]}; do
    fileName=$(echo "${fileToDownload}" | jq .fileName -r)
    downloadLocation=$(echo "${fileToDownload}" | jq .downloadLocation -r)
    versions=$(echo "${fileToDownload}" | jq .versions -r | jq -r ".[]")
    download_URL=$(echo "${fileToDownload}" | jq .downloadURL -r)
    targetContainerRuntime=$(echo "${fileToDownload}" | jq .targetContainerRuntime -r)
    if [ "${targetContainerRuntime}" != "null" ] && [ "${containerRuntime}" != "${targetContainerRuntime}" ]; then
      echo "$test: skipping ${fileName} verification as VHD container runtime is ${containerRuntime}, not ${targetContainerRuntime}"
      continue
    fi
    if [ ! -d $downloadLocation ]; then
      err $test "Directory ${downloadLocation} does not exist"
      continue
    fi

    for version in ${versions}; do
      file_Name=$(string_replace $fileName $version)
      dest="$downloadLocation/${file_Name}"
      downloadURL=$(string_replace $download_URL $version)/$file_Name
      if [ ! -s $dest ]; then
        err $test "File ${dest} does not exist"
        continue
      fi
      # no wc -c on a dir. This is for downloads we've un tar'd and deleted from the vhd
      if [ ! -d $dest ]; then
        # -L since some urls are redirects (i.e github)
        fileSizeInRepo=$(curl -sLI $downloadURL | grep -i Content-Length | tail -n1 | awk '{print $2}' | tr -d '\r')
        fileSizeDownloaded=$(wc -c $dest | awk '{print $1}' | tr -d '\r')
        if [[ "$fileSizeInRepo" != "$fileSizeDownloaded" ]]; then
          err $test "File size of ${dest} from ${downloadURL} is invalid. Expected file size: ${fileSizeInRepo} - downlaoded file size: ${fileSizeDownloaded}"
          continue
        fi
        # Validate whether package exists in Azure China cloud
        if [[ $downloadURL == https://acs-mirror.azureedge.net/* ]]; then
          mcURL="${downloadURL/https:\/\/acs-mirror.azureedge.net/https:\/\/kubernetesartifacts.blob.core.chinacloudapi.cn}"
          echo "Validating: $mcURL"
          isExist=$(curl -sLI $mcURL | grep -i "404 The specified blob does not exist." | awk '{print $2}')
          if [[ "$isExist" == "404" ]]; then
            err "$mcURL is invalid"
            continue
          fi

          fileSizeInMC=$(curl -sLI $mcURL | grep -i Content-Length | tail -n1 | awk '{print $2}' | tr -d '\r')
          if [[ "$fileSizeInMC" != "$fileSizeDownloaded" ]]; then
            err "$mcURL is valid but the file size is different. Expected file size: ${fileSizeDownloaded} - downlaoded file size: ${fileSizeInMC}"
            continue
          fi
        fi
      fi
    done

    echo "---"
  done
  echo "$test:Finish"
}

testImagesPulled() {
  test="testImagesPulled"
  echo "$test:Start"
  containerRuntime=$1
  if [ $containerRuntime == 'containerd' ]; then
    pulledImages=$(ctr -n k8s.io image ls)
  elif [ $containerRuntime == 'docker' ]; then
    pulledImages=$(docker images --format "{{.Repository}}:{{.Tag}}")
  else
    err $test "unsupported container runtime $containerRuntime"
    return
  fi

  imagesToBePulled=$(echo $2 | jq .ContainerImages[] --monochrome-output --compact-output)

  for imageToBePulled in ${imagesToBePulled[*]}; do
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
      download_URL=$(string_replace $downloadURL $version)

      if [[ $pulledImages =~ $downloadURL ]]; then
        echo "Image ${download_URL} pulled"
      else
        err $test "Image ${download_URL} has NOT been pulled"
      fi
    done

    echo "---"
  done
  echo "$test:Finish"
}

# check all the mcr images retagged for mooncake
testImagesRetagged() {
  containerRuntime=$1
  if [ $containerRuntime == 'containerd' ]; then
    # shellcheck disable=SC2207
    pulledImages=($(ctr -n k8s.io image ls))
  elif [ $containerRuntime == 'docker' ]; then
    # shellcheck disable=SC2207
    pulledImages=($(docker images --format "{{.Repository}}:{{.Tag}}"))
  else
    err $test "unsupported container runtime $containerRuntime"
    return
  fi
  mcrImagesNumber=0
  mooncakeMcrImagesNumber=0
  for pulledImage in ${pulledImages[@]}; do
    if [[ $pulledImage == "mcr.microsoft.com"* ]]; then
      mcrImagesNumber=$((${mcrImagesNumber} + 1))
    fi
    if [[ $pulledImage == "mcr.azk8s.cn"* ]]; then
      mooncakeMcrImagesNumber=$((${mooncakeMcrImagesNumber} + 1))
    fi
  done
  if [[ "${mcrImagesNumber}" != "${mooncakeMcrImagesNumber}" ]]; then
    echo "the number of the mcr images & mooncake mcr images are not the same."
    echo "all the images are:"
    echo "${pulledImages[@]}"
    exit 1
  fi
}

testAuditDNotPresent() {
  test="testAuditDNotPresent"
  echo "$test:Start"
  status=$(systemctl show -p SubState --value auditd.service)
  if [ $status == 'dead' ]; then
    echo "AuditD is not present, as expected"
  else
    err $test "AuditD is active with status ${status}"
  fi
  echo "$test:Finish"
}

testChrony() {
  os_sku=$1
  test="testChrony"
  echo "$test:Start"

  # ---- Test Setup ----
  # Test ntp is not active
  status=$(systemctl show -p SubState --value ntp)
  if [ $status == 'dead' ]; then
    echo $test "ntp is removed, as expected"
  else
    err $test "ntp is active with status ${status}"
  fi
  #test chrony is running
  #if mariner check chronyd, else check chrony
  os_chrony="chrony"
  if [[ "$os_sku" == "CBLMariner" ]]; then
    os_chrony="chronyd"
  fi
  status=$(systemctl show -p SubState --value $os_chrony)
  if [ $status == 'running' ]; then
    echo $test "$os_chrony is running, as expected"
  else
    err $test "$os_chrony is not running with status ${status}"
  fi

  #test if chrony corrects time
  if [ $os_sku == 'CBLMariner' ]; then
    echo $test "exiting without checking chrony time correction"
    echo $test "reenable after Mariner updates the chrony config in base image"
    echo "$test:Finish"
    return
  fi
  initialDate=$(date +%s)
  date --set "27 Feb 2021"
  for i in $(seq 1 10); do
    newDate=$(date +%s)
    if (($newDate > $initialDate)); then
      echo "$os_chrony readjusted the system time correctly"
      break
    fi
    sleep 10
    echo "${i}: retrying: check if chrony modified the time"
  done
  if (($i == 10)); then
    err $test "$os_chrony failed to readjust the system time"
  fi
  echo "$test:Finish"
}

testFips() {
  test="testFips"
  echo "$test:Start"
  os_version=$1
  enable_fips=$2

  if [[ (${os_version} == "18.04" || ${os_version} == "20.04") && ${enable_fips,,} == "true" ]]; then
    kernel=$(uname -r)
    if [[ -f /proc/sys/crypto/fips_enabled ]]; then
      fips_enabled=$(cat /proc/sys/crypto/fips_enabled)
      if [[ "${fips_enabled}" == "1" ]]; then
        echo "FIPS is enabled."
      else
        err $test "content of /proc/sys/crypto/fips_enabled is not 1."
      fi
    else
      err $test "FIPS is not enabled."
    fi

    if [[ -f /usr/src/linux-headers-${kernel}/Makefile ]]; then
      echo "fips header files exist."
    else
      err $test "fips header files don't exist."
    fi
  fi

  echo "$test:Finish"
}

testKubeBinariesPresent() {
  test="testKubeBinaries"
  echo "$test:Start"
  containerRuntime=$1
  binaryDir=/usr/local/bin
  k8sVersions="$(jq -r .kubernetes.versions[] </opt/azure/manifest.json)"
  for patchedK8sVersion in ${k8sVersions}; do
    # Only need to store k8s components >= 1.19 for containerd VHDs
    if (($(echo ${patchedK8sVersion} | cut -d"." -f2) < 19)) && [[ ${containerRuntime} == "containerd" ]]; then
      continue
    fi
    # strip the last .1 as that is for base image patch for hyperkube
    if grep -iq hotfix <<<${patchedK8sVersion}; then
      # shellcheck disable=SC2006
      patchedK8sVersion=$(echo ${patchedK8sVersion} | cut -d"." -f1,2,3,4)
    else
      patchedK8sVersion=$(echo ${patchedK8sVersion} | cut -d"." -f1,2,3)
    fi
    k8sVersion=$(echo ${patchedK8sVersion} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
    kubeletDownloadLocation="$binaryDir/kubelet-$k8sVersion"
    kubectlDownloadLocation="$binaryDir/kubectl-$k8sVersion"
    kubeletInstallLocation="/usr/local/bin/kubelet"
    kubectlInstallLocation="/usr/local/bin/kubectl"
    #Test whether the binaries have been extracted
    if [ ! -s $kubeletDownloadLocation ]; then
      err $test "Binary ${kubeletDownloadLocation} does not exist"
    fi
    if [ ! -s $kubectlDownloadLocation ]; then
      err $test "Binary ${kubectlDownloadLocation} does not exist"
    fi
    #Test whether the installed binary version is indeed correct
    mv $kubeletDownloadLocation $kubeletInstallLocation
    mv $kubectlDownloadLocation $kubectlInstallLocation
    chmod a+x $kubeletInstallLocation $kubectlInstallLocation
    echo "kubectl version"
    kubectlLongVersion=$(kubectl version 2>/dev/null)
    if [[ ! $kubectlLongVersion =~ $k8sVersion ]]; then
      err $test "The kubectl version is not correct: expected kubectl version $k8sVersion existing: $kubectlLongVersion"
    fi
    echo "kubelet version"
    kubeletLongVersion=$(kubelet --version 2>/dev/null)
    if [[ ! $kubeletLongVersion =~ $k8sVersion ]]; then
      err $test "The kubelet version is not correct: expected kubelet version $k8sVersion existing: $kubeletLongVersion"
    fi
  done
  echo "$test:Finish"
}

testKubeProxyImagesPulled() {
  test="testKubeProxyImagesPulled"
  echo "$test:Start"
  containerRuntime=$1
  containerdKubeProxyImages=$(jq .containerdKubeProxyImages <${KUBE_PROXY_IMAGES_FILEPATH})

  if [ $containerRuntime == 'containerd' ]; then
    testImagesPulled containerd "$containerdKubeProxyImages"
  else
    err $test "unsupported container runtime $containerRuntime"
    return
  fi
  echo "$test:Finish"
}

# nc and nslookup is used in CSE to check connectivity
testCriticalTools() {
  test="testCriticalTools"
  echo "$test:Start"
  if ! nc -h 2>/dev/null; then
    err $test "nc is not installed"
  else
    echo $test "nc is installed"
  fi

  if ! nslookup -version 2>/dev/null; then
    err $test "nslookup is not installed"
  else
    echo $test "nslookup is installed"
  fi

  echo "$test:Finish"
}

testCustomCAScriptExecutable() {
  test="testCustomCAScriptExecutable"
  permissions=$(stat -c "%a" /opt/scripts/update_certs.sh)
  if [ "$permissions" != "755" ]; then
    err $test "/opt/scripts/update_certs.sh has incorrect permissions"
  fi
  echo "$test:Finish"
}

testCustomCATimerNotStarted() {
  isUnitThere=$(systemctl list-units --type=timer | grep update_certs.timer)
  if [[ -n "$isUnitThere" ]]; then
    err $test "Custom CA timer was loaded, but shouldn't be"
  fi
  echo "$test:Finish"
}

testVHDBuildLogsExist() {
  test="testVHDBuildLogsExist"
  if [ -f $VHD_LOGS_FILEPATH ]; then
    echo "detected vhd logs file"
  else
    err $test "File $VHD_LOGS_FILEPATH not found"
    exit $ERR_VHD_FILE_NOT_FOUND
  fi
  echo "$test:Finish"
}

# Ensures that /etc/login.defs is valid. This is a best-effort test, as we aren't going to
# re-implement everything that uses this file.
testLoginDefs() {
  test="testLoginDefs"
  local settings_file=/etc/login.defs
  echo "$test:Start"

  # Existence and format check. Based on https://man7.org/linux/man-pages/man5/login.defs.5.html,
  # we expect the file to have lines that are either a comment or "NAME VALUE" pairs. Arbitrary whitespace
  # is allowed before NAME and between NAME and VALUE. NAME seems tobe upper-case and '_'.
  # all-caps and include letters and '_'. Values can be anything, so we make sure they're printable.
  testSettingFileFormat $test $settings_file '^[[:space:]]*(#|$)' '^[[:space:]]*[A-Z_]+[[:space:]]+[^[:cntrl:]]+$'

  # Look for the settings we specifically set in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh
  # and ensure they're set to the values we expect.
  echo "$test: Checking specific settings in $settings_file"
  testSetting $test $settings_file PASS_MAX_DAYS '^[[:space:]]*PASS_MAX_DAYS[[:space:]]' ' ' 90
  testSetting $test $settings_file PASS_MIN_DAYS '^[[:space:]]*PASS_MIN_DAYS[[:space:]]+' ' ' 7

  echo "$test:Finish"
}

# Ensures that /etc/default/useradd is valid. This is a best-effort test, as we aren't going to
# re-implement everything that uses this file.
testUserAdd() {
  test="testUserAdd"
  local settings_file=/etc/default/useradd
  echo "$test:Start"

  # Existence and format check. The man page https://www.man7.org/linux/man-pages/man8/useradd.8.html
  # doesn't really state the format of the file, but experimentation and examples show that each
  # line must be a comment or 'NAME=VALUE', where values can be empty or strings, and strings
  # is pretty loose (any printable character).
  testSettingFileFormat $test $settings_file '^[[:space:]]*(#|$)' '^[A-Z_]+=[^[:cntrl:]]*$'

  # Look for the settings we specifically set in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh
  # and ensure they're set to the values we expect.
  echo "$test: Checking specific settings in $settings_file"
  testSetting $test $settings_file INACTIVE '^INACTIVE=' '=' 30

  # Double-check that the setting we set is actually used by useradd.
  # Disable shellcheck warning about using '$?' in an if statement because we don't want
  # the return value anyway and the ways it suggests reorganizing the if statement
  # actually confuse it even more.
  echo "$test: Checking that INACTIVE is used by useradd"
  useradd -D | grep -E -v "^INACTIVE=30$" >/dev/null
  # shellcheck disable=SC2181
  if [ $? -ne 0 ]; then
    err $test "useradd is not using INACTIVE=30 from $settings_file"
  fi
  echo "$test: useradd is using INACTIVE=30 from $settings_file"

  echo "$test:Finish"
}

testNetworkSettings() {
  local test="testNetworkSettings"
  local settings_file=/etc/sysctl.d/60-CIS.conf
  echo "$test:Start"

  # Existence and format check. Based on the man page https://www.man7.org/linux/man-pages/man5/sysctl.conf.5.html
  # we expect the file to have lines that are either a comment or "NAME = VALUE" pairs. Arbitrary whitespace
  # is allowed before NAME and between NAME and VALUE. It can also just be "NAME" (no '='). Name seems to be
  # lower-case and include letters and '_' and '.'. Value can be anything, so we make sure they're printable.
  # If a line starts with '-', it has special meaning so we need to allow that too.
  testSettingFileFormat $test $settings_file '^[[:space:]]*(#|;|$)' '^-{0,1}[[:space:]]*[a-z\.0-9_\*]+[[:space:]]*$' '^-{0,1}[[:space:]]*[a-z\.0-9_\*]+[[:space:]]*=[[:space:]]*[^[:cntrl:]]*$'

  echo "$test:End"
}

# Tests a setting file's format. This is a simple, line-by line check.
# Parameters:
#  test: The name of the test.
#  settings_file: The path to the settings file.
#  Remaining parameters are regexes used for validation.
#
#  The file will be tested for existence, then each line of the file is expected
#  to match at least one of the regexes.
#
#  Any lines that match none of the regexes are printed to stderr.
#  Returns 0 if all lines are valid, 1 otherwise.
testSettingFileFormat() {
  local test="$1"
  local settings_file="$2"
  shift 2

  # If the file doesn't exist, everything is broken.
  echo "$test: Checking existence of $settings_file"
  if [ ! -f $settings_file ]; then
    err $test "File $settings_file not found"
    return 1
  fi
  echo "$test: $settings_file exists"

  # Loop through each line in the file.
  # For each line, each regex is checked. If none match, the line is invalid.
  echo "$test: Checking format of $settings_file"
  local line_num=1
  local line
  local regex
  local valid=0
  local any_invalid=0
  while read -r line; do
    line_num=$((line_num + 1))
    for regex in "$@"; do
      if [[ "$line" =~ $regex ]]; then
        valid=1
        break
      fi
    done

    if [ $valid -eq 0 ]; then
      any_invalid=1
      err $test "Invalid line $line_num in $settings_file: '$line'" >>/dev/stderr
    fi

    valid=0
  done <$settings_file

  if [ $any_invalid -eq 0 ]; then
    echo "$test: $settings_file is valid"
  fi

  return $any_invalid
}

# Tests an individual setting in a settings file, ensuring it's set with the correct value.
# Note: This assumes the file format is generally correct (see function testSettingFileFormat).
# Parameters:
#  test: The name of the test.
#  settings_file: The path to the settings file.
#  setting_name: The name of the setting to check.
#  setting_line_regex: A regex that matches the line that contains the setting. This should be
#                      setting-specific -- if you want to check setting 'FOO', this should look
#                      specifically for 'FOO' in the line.
#  setting_value_awk_separator: The separator to use when parsing the setting value from the line.
#                               This is used in an awk command, so it should be a single character.
#  expected_value: The expected value of the setting.
testSetting() {
  local test="$1"
  local settings_file="$2"
  local setting_name="$3"
  local setting_line_regex="$4"
  local setting_value_awk_separator="$5"
  local expected_value="$6"

  echo "$test: Checking setting '$setting_name' has value '$expected_value' in $settings_file"
  # Get the lines that match the setting name. Note that this will come with the line number.
  local value_lines=
  value_lines=$(grep -E -n "${setting_line_regex}" "${settings_file}")

  # If the setting isn't present, that's an error.
  if [ -z "$value_lines" ]; then
    err $test "Setting '$setting_name' not found in $settings_file"
    return 1
  fi

  # If the setting is present more than once, that's an error.
  if [ $(echo "$value_lines" | wc -l) -gt 1 ]; then
    err $test "Setting '$setting_name' found more than once in $settings_file. See below for lines."
    echo "$value_lines" >>/dev/stderr
    return 1
  fi

  # Get the value of the setting and test it. To do this we must strip out the line number. We also
  # trim leading and trailing whitespace around the value with xargs.
  local value=
  value=$(echo "$value_lines" | sed -E 's/^([0-9]+:)//' | awk -F "$setting_value_awk_separator" '{print $2}' | xargs)
  if [ "$value" != "$expected_value" ]; then
    err $test "Setting '$setting_name' has value '$value' in $settings_file, expected '$expected_value'"
    return 1
  fi

  echo "$test: Setting '$setting_name' has value correct value '$expected_value' in $settings_file"
  return 0
}

err() {
  echo "$1:Error: $2" >>/dev/stderr
}

string_replace() {
  echo ${1//\*/$2}
}

testVHDBuildLogsExist
testCriticalTools
testFilesDownloaded $CONTAINER_RUNTIME
testImagesPulled $CONTAINER_RUNTIME "$(cat $COMPONENTS_FILEPATH)"
testChrony $OS_SKU
testAuditDNotPresent
testFips $OS_VERSION $ENABLE_FIPS
testKubeBinariesPresent $CONTAINER_RUNTIME
testKubeProxyImagesPulled $CONTAINER_RUNTIME
testImagesRetagged $CONTAINER_RUNTIME
testCustomCAScriptExecutable
testCustomCATimerNotStarted
testLoginDefs
testUserAdd
testNetworkSettings
