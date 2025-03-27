#!/bin/bash
COMPONENTS_FILEPATH=/opt/azure/components.json
MANIFEST_FILEPATH=/opt/azure/manifest.json
VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
AZURELINUX_OS_NAME="AZURELINUX"

THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"
CONTAINER_RUNTIME="$1"
OS_VERSION="$2"
ENABLE_FIPS="$3"
OS_SKU="$4"
GIT_BRANCH="$5"
IMG_SKU="$6"
FEATURE_FLAGS="$7"

# List of "ERROR/WARNING" message we want to ignore in the cloud-init.log
# 1. "Command ['hostname', '-f']":
#   Running hostname -f will fail on current AzureLinux AKS image. We don't not have active plan to resolve
#   this for stable version and there is no customer issues collected. Ignore this failure now.
CLOUD_INIT_LOG_MSG_IGNORE_LIST=(
  "Command ['hostname', '-f']"
)

err() {
  echo "$1:Error: $2" >>/dev/stderr
}

# Clone the repo and checkout the branch provided.
# Simply clone with just the branch doesn't work for pull requests, but this technique works
# with everything we've tested so far.
#
# Strategy is to clone the repo, fetch the remote branch by ref into a local branch, and then checkout the local branch.
# The remote branch will be something like 'refs/heads/branch/name' or 'refs/pull/number/head'. Using the same name
# for the local branch has weird semantics, so we replace '/' with '-' for the local branch name.
LOCAL_GIT_BRANCH=${GIT_BRANCH//\//-}
echo "Cloning AgentBaker repo and checking out remote branch '${GIT_BRANCH}' into local branch '${LOCAL_GIT_BRANCH}'"
COMMAND="git clone --quiet https://github.com/Azure/AgentBaker.git"
if ! ${COMMAND}; then
  err 'git-clone' "Failed to clone AgentBaker repo"
  err 'git-clone' "Used command '${COMMAND}'"
  exit 1
fi
if ! pushd ./AgentBaker; then
  err 'git-clone' "Failed to pushd into AgentBaker repo -- this is weird given that clone succeeded"
  err 'git-clone' "Current directory is '$(pwd)'"
  err 'git-clone' "Contents of current directory: $(ls -al)"
  exit 1
fi
COMMAND="git fetch --quiet origin ${GIT_BRANCH}:${LOCAL_GIT_BRANCH}"
if ! ${COMMAND}; then
  err 'git-clone' "Failed to fetch remote branch '${GIT_BRANCH}' into local branch '${LOCAL_GIT_BRANCH}'"
  err 'git-clone' "Used command '${COMMAND}'"
  exit 1
fi
COMMAND="git checkout --quiet ${LOCAL_GIT_BRANCH}"
if ! ${COMMAND}; then
  err 'git-clone' "Failed to checkout local branch '${LOCAL_GIT_BRANCH}'"
  err 'git-clone' "Used command '${COMMAND}'"
  exit 1
fi
if ! popd; then
  err 'git-clone' "Failed to popd out of AgentBaker repo -- this seems impossible"
  err 'git-clone' "Current directory is $(pwd)"
  err 'git-clone' "pushd stack is $(dirs -p)"
  exit 1
fi

source ./AgentBaker/parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh 2>/dev/null
source ./AgentBaker/parts/linux/cloud-init/artifacts/cse_helpers.sh 2>/dev/null

validateDownloadPackage() {
  local downloadURL=$1
  local downloadedPackage=$2
  fileSizeInRepo=$(curl -sLI $downloadURL | grep -i Content-Length | tail -n1 | awk '{print $2}' | tr -d '\r')
  fileSizeDownloaded=$(wc -c $downloadedPackage | awk '{print $1}' | tr -d '\r')
  if [[ "$fileSizeInRepo" != "$fileSizeDownloaded" ]]; then
    return 1
  fi
  return 0
}

validateOrasOCIArtifact() {
  local downloadURL=$1
  local downloadedPackage=$2
  echo "Validating package $downloadURL from registry and downloaded package $downloadedPackage"

  # Fetch the manifest and extract the size using jq
  fileSizeInRegistry=$(oras manifest fetch --registry-config ${ORAS_REGISTRY_CONFIG_FILE} "$downloadURL" | jq '.layers[0].size')
  
  # Get the size of the downloaded package
  fileSizeDownloaded=$(wc -c "$downloadedPackage" | awk '{print $1}' | tr -d '\r')
  
  # Compare the sizes
  if [[ "$fileSizeInRegistry" != "$fileSizeDownloaded" ]]; then
    echo "Error: File size mismatch. Expected $fileSizeInRegistry, but got $fileSizeDownloaded."
    return 1
  fi

  echo "Package validated successfully."
  return 0
}

testAcrCredentialProviderInstalled() {
  test="testAcrCredentialProviderInstalled"
  echo "$test:Start"
  local downloadURL=$1
  local acrCredProviderVersions=("${@:2}")
  for version in "${acrCredProviderVersions[@]}"; do
    echo "checking acrCredProviderVersions: $version ..."
    eval "currentDownloadURL=${downloadURL}"

    # if currentDownloadURL is mcr.microsoft.com/oss/binaries/kubernetes/azure-acr-credential-provider:v1.30.0-linux-amd64,
    # then downloadLocation should be /opt/credentialprovider/downloads/azure-acr-credential-provider-linux-amd64-v1.30.0.tar.gz
    downloadLocation="/opt/credentialprovider/downloads/azure-acr-credential-provider-linux-${CPU_ARCH}-${version}.tar.gz"
    validateOrasOCIArtifact $currentDownloadURL $downloadLocation
    if [[ $? -ne 0 ]]; then
      err $test "File size of ${downloadLocation} from ${currentDownloadURL} is invalid. Expected file size: ${fileSizeInRepo} - downloaded file size: ${fileSizeDownloaded}"
      continue
    fi
  done
  echo "$test:Finish"
}

testPackagesInstalled() {
  test="testPackagesInstalled"
  containerRuntime=$1
  if [[ $(isARM64) == 1 ]]; then
    return
  fi
  CPU_ARCH="amd64"
  echo "$test:Start"
  packages=$(jq ".Packages" $COMPONENTS_FILEPATH | jq .[] --monochrome-output --compact-output)

  while IFS= read -r p; do
    name=$(echo "${p}" | jq .name -r)
    downloadLocation=$(echo "${p}" | jq .downloadLocation -r)
    if [[ "$downloadLocation" == "" ]]; then
      continue
    fi
    if [[ "$OS_SKU" == "CBLMariner" || ("$OS_SKU" == "AzureLinux" && "$OS_VERSION" == "2.0") ]]; then
      OS=$MARINER_OS_NAME
    elif [[ "$OS_SKU" == "AzureLinux" && "$OS_VERSION" == "3.0" ]]; then
      OS=$AZURELINUX_OS_NAME
    else
      OS=$UBUNTU_OS_NAME
    fi
    PACKAGE_VERSIONS=()
    updatePackageVersions "${p}" "${OS}" "${OS_VERSION}"
    PACKAGE_DOWNLOAD_URL=""
    updatePackageDownloadURL "${p}" "${OS}" "${OS_VERSION}"
    if [ "${name}" == "kubernetes-binaries" ]; then
      # kubernetes-binaries, namely, kubelet and kubectl are installed in a different way so we test them separately
      # Intentionally remove leading 'v' from each element in the array
      testKubeBinariesPresent "${PACKAGE_VERSIONS[@]#v}"
      continue
    fi
    if [ "${name}" == "azure-acr-credential-provider" ]; then
      # azure-acr-credential-provider is installed in a different way so we test it separately
      testAcrCredentialProviderInstalled "$PACKAGE_DOWNLOAD_URL" "${PACKAGE_VERSIONS[@]}"
      continue
    fi

    for version in "${PACKAGE_VERSIONS[@]}"; do
      if [[ -z $PACKAGE_DOWNLOAD_URL ]]; then
        echo "$test: skipping package ${name} verification as PACKAGE_DOWNLOAD_URL is empty"
        # we can further think of adding a check to see if the package is installed through apt-get
        break
      fi
      # A downloadURL from a package in components.json will look like this: 
      # "https://acs-mirror.azureedge.net/cni-plugins/v${version}/binaries/cni-plugins-linux-${CPU_ARCH}-v${version}.tgz"
      # After eval(resolved), downloadURL will look like "https://acs-mirror.azureedge.net/cni-plugins/v0.8.7/binaries/cni-plugins-linux-arm64-v0.8.7.tgz"
      eval "downloadURL=${PACKAGE_DOWNLOAD_URL}"
      local fileNameWithExt
      fileNameWithExt=$(basename $downloadURL)
      local fileNameWithoutExt
      fileNameWithoutExt="${fileNameWithExt%.*}"
      local downloadedPackage
      downloadedPackage="$downloadLocation/${fileNameWithExt}"
      local extractedPackageDir
      extractedPackageDir="$downloadLocation/${fileNameWithoutExt}"

      # Validate whether package proxy path exists in Azure China cloud.
      if [[ $downloadURL == https://acs-mirror.azureedge.net/* ]]; then
        testPackageInAzureChinaCloud "$downloadURL"
      fi

      # if there is a directory with expected name, we assume it's been downloaded and extracted properly
      # no wc (wordcount) -c on a dir. This is for downloads we've un tar'd and deleted from the vhd
      if [ -d "$extractedPackageDir" ]; then
        echo $test "[INFO] Directory ${extractedPackageDir} exists"
        continue
      fi

      # if the downloadLocation is /usr/local/bin verify that the package is installed
      if [ "$downloadLocation" == "/usr/local/bin" ]; then
        if command -v "$name" >/dev/null 2>&1; then
          echo "$name is installed."
          continue
        elif [ "$name" == "containerd-wasm-shims" ]; then
          testWasmRuntimesInstalled $downloadLocation $version
          echo "$test $name binaries are in the expected location of $downloadLocation"
          continue
        elif [ "$name" == "spinkube" ]; then
          testSpinKubeInstalled $downloadLocation $version
          echo "$test $name binaries are in the expected location of $downloadLocation"
          continue
        else
          err $test "$name is not installed. Expected to be installed in $downloadLocation"
          continue
        fi
      fi

      # if there isn't a directory, we check if the file exists and the size is correct
      # -L since some urls are redirects (i.e github)
      # shellcheck disable=SC2086
      validateDownloadPackage "$downloadURL" $downloadedPackage
      if [[ $? -ne 0 ]]; then
        err $test "File size of ${downloadedPackage} from ${downloadURL} is invalid. Expected file size: ${fileSizeInRepo} - downloaded file size: ${fileSizeDownloaded}"
        continue
      fi
      echo $test "[INFO] File ${downloadedPackage} exists and has the correct size ${fileSizeDownloaded} bytes"

    done

    echo "---"
  done <<<"$packages"
  echo "$test:Finish"
}

# Azure China Cloud uses a different proxy but the same path, and we want to verify the package URL
# if defined in control plane, is accessible and has the same file size as the one in the public cloud.
testPackageInAzureChinaCloud() {
  # In Azure China Cloud, the proxy server proxies download URL to the storage account URL according to the root path, for example, 
  # location /kubernetes/ {
  #  proxy_pass https://kubernetesartifacts.blob.core.chinacloudapi.cn/kubernetes/;
  # }

  local downloadURL=$1

  proxyLocation=$(echo "$downloadURL" | awk -F'/' '{print $4}')

  # root paths like cri-tools can be ignored since they are only cached in VHD and won't be referenced in control plane.
  rootPathExceptions=("cri-tools" "spinkube")
  for rootPathException in "${rootPathExceptions[@]}"; do
    if [ "$rootPathException" == "$proxyLocation" ]; then
      return
    fi
  done

  supportedProxyLocations=(
    "aks"
    "kubernetes"
    "cni-plugins"
    "azure-cni"
    "csi-proxy"
    "aks-engine"
    "containerd"
    "calico-node"
    "ccgakvplugin"
    "cloud-provider-azure"
    )

  foundLocation=false
  for supportedProxyLocation in "${supportedProxyLocations[@]}"; do
    if [ "$supportedProxyLocation" == "$proxyLocation" ]; then
      foundLocation=true
      break
    fi
  done

  if [ "$foundLocation" == false ]; then
    err "Proxy location $proxyLocation is not defined in mooncake for $downloadURL, please use root path 'aks' , or contact 'andyzhangx' for help"
    return
  fi

  mcURL="${downloadURL/https:\/\/acs-mirror.azureedge.net/https:\/\/kubernetesartifacts.blob.core.chinacloudapi.cn}"
  echo "Validating: $mcURL"
  isExist=$(curl -sLI "$mcURL" | grep -i "404 The specified blob does not exist." | awk '{print $2}')
  if [[ "$isExist" == "404" ]]; then
    err "$mcURL is invalid"
    return
  fi

  fileSizeInMC=$(curl -sLI $mcURL | grep -i Content-Length | tail -n1 | awk '{print $2}' | tr -d '\r')
  fileSizeInRepo=$(curl -sLI $downloadURL | grep -i Content-Length | tail -n1 | awk '{print $2}' | tr -d '\r')


  if [[ "$fileSizeInMC" != "$fileSizeInRepo" ]]; then
    err "$mcURL is valid but the file size is different. Expected file size: ${fileSizeDownloaded} - file size in Mooncake: ${fileSizeInMC}"
    return
  fi

}

testImagesPulled() {
  test="testImagesPulled"
  local componentsJsonContent="$2"
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

  imagesToBePulled=$(echo "${componentsJsonContent}" | jq .ContainerImages[] --monochrome-output --compact-output)

  while IFS= read -r imageToBePulled; do
    downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
    amd64OnlyVersionsStr=$(echo "${imageToBePulled}" | jq .amd64OnlyVersions -r)
    MULTI_ARCH_VERSIONS=()
    updateMultiArchVersions "${imageToBePulled}"

    amd64OnlyVersions=""
    if [[ ${amd64OnlyVersionsStr} != null ]]; then
      amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
    fi

    if [[ $(isARM64) == 1 ]]; then
      versions="${MULTI_ARCH_VERSIONS}"
    else
      versions="${amd64OnlyVersions} ${MULTI_ARCH_VERSIONS}"
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
  done <<<"$imagesToBePulled"
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
  while IFS= read -r pulledImage; do
    if [[ $pulledImage == "mcr.microsoft.com"* ]]; then
      mcrImagesNumber=$((${mcrImagesNumber} + 1))
    fi
    if [[ $pulledImage == "mcr.azk8s.cn"* ]]; then
      mooncakeMcrImagesNumber=$((${mooncakeMcrImagesNumber} + 1))
    fi
  done <<<"$pulledImages"
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
  #if mariner/azurelinux check chronyd, else check chrony
  os_chrony="chrony"
  if [[ "$os_sku" == "CBLMariner" || "$os_sku" == "AzureLinux" ]]; then
    os_chrony="chronyd"
  fi
  status=$(systemctl show -p SubState --value $os_chrony)
  if [ $status == 'running' ]; then
    echo $test "$os_chrony is running, as expected"
  else
    err $test "$os_chrony is not running with status ${status}"
  fi

  #test if chrony corrects time
  if [[ "$os_sku" == "CBLMariner" || "$os_sku" == "AzureLinux" ]]; then
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

  if [[ (${os_version} == "18.04" || ${os_version} == "20.04" || ${os_version} == "22.04" || ${os_version} == "V2") && ${enable_fips,,} == "true" ]]; then
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

    if [[ ${os_version} == "18.04" || ${os_version} == "20.04" ]]; then
      if [[ -f /usr/src/linux-headers-${kernel}/Makefile ]]; then
        echo "fips header files exist."
      else
        err $test "fips header files don't exist."
      fi
    fi
  fi

  echo "$test:Finish"
}

testLtsKernel() {
  test="testLtsKernel"
  echo "$test:Start"
  os_version=$1
  os_sku=$2
  enable_fips=$3

  if [[ "$os_sku" == "Ubuntu" && ${enable_fips,,} != "true" ]]; then
    echo "OS is Ubuntu and FIPS is not enabled, check LTS kernel version"
    # Check the Ubuntu version and set the expected kernel version
    if [[ "$os_version" == "2204" ]]; then
      expected_kernel="5.15"
    elif [[ "$os_version" == "2404" ]]; then
      expected_kernel="6.8"
    else
      echo "LTS kernel not installed for: $os_version"
    fi

    kernel=$(uname -r)
    echo "Current kernel version: $kernel"
    if [[ "$kernel" == *"$expected_kernel"* ]]; then
      echo "Kernel version is as expected ($expected_kernel)."
    else
      echo "Kernel version is not as expected. Expected $expected_kernel, found $kernel."
    fi
  else
    echo "OS is not Ubuntu OR OS is Ubuntu and FIPS is true, skip LTS kernel test"
  fi

}

testCloudInit() {
  test="testCloudInit"
  echo "$test:Start"
  os_sku=$1

  # Limit this test only to non-cvm Mariner or Azurelinux
  if ! grep -q "cvm" <<< "$FEATURE_FLAGS" && [[ "${os_sku}" == "CBLMariner" || "${os_sku}" == "AzureLinux" ]]; then
    echo "Checking if cloud-init.log exists..."
    FILE=/var/log/cloud-init.log
    if test -f "$FILE"; then
      echo "Cloud-init log exists. Checking its content..."
      grep 'WARNING\|ERROR' $FILE | while read -r msg; do
        for pattern in "${CLOUD_INIT_LOG_MSG_IGNORE_LIST[@]}"; do
            if [[ "$msg" == *"$pattern"* ]]; then
                echo "Ignoring WARNING/ERROR message from ignore list; '${msg}'"
            else
                err $test "Cloud-init log has unexpected WARNING/ERROR: '${msg}'"
            fi
        done
      done
      echo "Cloud-init log is OK."
    else
      err $test "Check cloud-init log does not exist."
    fi

    echo "Checking cloud-init status..."
    cloud_init_output=$(cloud-init status --wait)
    cloud_init_status=$?
    if [ ${cloud_init_status} -eq 0 ]; then
      echo "Cloud-init status is OK."
    else
      err $test "Cloud-init exit status with code ${cloud_init_status}, ${cloud_init_output}."
    fi
  fi

  echo "$test:Finish"
}

testKubeBinariesPresent() {
  test="testKubeBinaries"
  echo "$test:Start"
  local kubeBinariesVersions=("$@")
  binaryDir=/usr/local/bin
  for patchedK8sVersion in "${kubeBinariesVersions[@]}"; do
    echo "checking kubeBinariesVersions: $patchedK8sVersion ..."
    # strip the last .1 as that is for base image patch for hyperkube
    if grep -iq hotfix <<<${patchedK8sVersion}; then
      # shellcheck disable=SC2006
      patchedK8sVersion=$(echo ${patchedK8sVersion} | cut -d"." -f1,2,3,4)
    else
      patchedK8sVersion=$(echo ${patchedK8sVersion} | cut -d"." -f1,2,3)
    fi
    k8sVersion=$(echo ${patchedK8sVersion} | cut -d"_" -f1 | cut -d"-" -f1 | cut -d"." -f1,2,3)
    if grep -iq akslts <<<${patchedK8sVersion}; then
      k8sVersion="$k8sVersion-akslts"
    fi
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
    chmod a+x $kubeletDownloadLocation $kubectlDownloadLocation
    kubectlLongVersion=$(${kubectlDownloadLocation} version 2>/dev/null)
    if [[ ! $kubectlLongVersion =~ $k8sVersion ]]; then
      err $test "The kubectl version is not correct: expected kubectl version $k8sVersion existing: $kubectlLongVersion"
    fi
    kubeletLongVersion=$(${kubeletDownloadLocation} --version 2>/dev/null)
    if [[ ! $kubeletLongVersion =~ $k8sVersion ]]; then
      err $test "The kubelet version is not correct: expected kubelet version $k8sVersion existing: $kubeletLongVersion"
    fi
  done
  echo "$test:Finish"
}

# nc and nslookup is used in CSE to check connectivity
testCriticalTools() {
  test="testCriticalTools"
  echo "$test:Start"

  #TODO (djsly): netcat is only required with 18.04, remove this check when 18.04 is deprecated
  if ! nc -h 2>/dev/null; then
    err $test "nc is not installed"
  else
    echo $test "nc is installed"
  fi

  if ! curl -h 2>/dev/null; then
    err $test "curl is not installed"
  else
    echo $test "curl is installed"
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

testCustomCATrustNodeCAWatcherRetagged() {
  isStaticTagImageThere=$(crictl images list | grep 'aks-node-ca-watcher' | grep 'static')
  if [[ -z "$isStaticTagImageThere" ]]; then
    err $test "Expected to find Node CA Watcher with static tag on the node"
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
  testSetting $test $settings_file UMASK '^[[:space:]]*UMASK[[:space:]]+' ' ' 027

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

# Ensures that the content /etc/profile.d/umask.sh is correct, per code in
# <repo-root>/parts/linux/cloud-init/artifacts/cis.sh
testUmaskSettings() {
    local test="testUmaskSettings"
    local settings_file=/etc/profile.d/umask.sh
    local expected_settings_file_content='umask 027'
    echo "$test:Start"

    # If the settings file exists, it must just be a single line that sets umask properly.
    if [[ -f "${settings_file}" ]]; then
        echo "${test}: Checking that the contents of ${settings_file} is exactly '${expected_settings_file_content}'"

        # Command substitution (like file_contents=$(cat "${settings_file}")) strips trailing newlines, so we use mapfile instead.
        # This creates an array of the lines in the file, and then we join them back together by expanding the array into a single string.
        local file_contents_array=()
        mapfile <"${settings_file}" file_contents_array
        local file_contents="${file_contents_array[*]}"
        if [[ "${file_contents}" != "${expected_settings_file_content}" ]]; then
            err $test "The content of the file '${settings_file}' is '${file_contents}', which does not exactly match '${expected_settings_file_content}'. "
        else
            echo "${test}: The content of the file '${settings_file}' exactly matches the expected contents '${expected_settings_file_content}'."
        fi
    else
        echo "${test}: Settings file '${settings_file}' does not exist, so not testing contents."
    fi

    echo "$test:End"
}

# Tests that the modes on the cron-related files and directories in /etc are set correctly, per the
# function assignFilePermissions in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh.
testCronPermissions() {
  local test="testCronPermissions"
  echo "$test:Start"

  image_sku="${1}"
  declare -A required_paths=(
    ['/etc/cron.allow']=640
    ['/etc/cron.hourly']=600
    ['/etc/cron.daily']=600
    ['/etc/cron.weekly']=600
    ['/etc/cron.monthly']=600
    ['/etc/cron.d']=600
  )

  declare -A optional_paths=(
    ['/etc/crontab']=600
  )

  declare -a disallowed_paths=(
    '/etc/cron.deny'
  )

  if [[ "${image_sku}" != *"minimal"* ]]; then
    echo "$test: Checking required paths"
    for path in "${!required_paths[@]}"; do
      checkPathPermissions $test $path ${required_paths[$path]} 1
    done

    echo "$test: Checking optional paths"
    for path in "${!optional_paths[@]}"; do
      checkPathPermissions $test $path ${optional_paths[$path]} 0
    done

    echo "$test: Checking disallowed paths"
    for path in "${disallowed_paths[@]}"; do
      checkPathDoesNotExist $test $path
    done
  else
    echo "$test: Skipping cron file check for Ubuntu Minimal images"
  fi

  echo "$test:Finish"
}

# Tests that /etc/systemd/coredump.conf is set correctly, per the function
# configureCoreDump in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh.
testCoreDumpSettings() {
  local test="testCoreDumpSettings"
  local settings_file=/etc/systemd/coredump.conf
  echo "$test:Start"

  # Existence and format check. The man page https://www.man7.org/linux/man-pages/man5/coredump.conf.5.html
  # doesn't really state the format of the file, but show that each line must be one o:
  #   A comment starting with '#'.
  #   A section heading -- this file is only supposed to have '[Coredump]'
  #   Settings, which take the form 'NAME=VALUE', where values can be empty or strings, and strings
  #   is pretty loose (more or less any printable character).
  testSettingFileFormat $test $settings_file '^(#|$)' '^\[Coredump\]$' '^[A-Za-z_]+=[^[:cntrl:]]*$'

  # Look for the settings we specifically set in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh
  # and ensure they're set to the values we expect.
  echo "$test: Checking specific settings in $settings_file"
  testSetting $test $settings_file 'Storage' '^Storage=' '=' 'none'
  testSetting $test $settings_file 'ProcessSizeMax' '^ProcessSizeMax=' '=' '0'

  echo "$test:Finish"
}

# Tests that the nfs-server systemd service is masked, per the function
# configuremaskNfsServerNfsServer in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh.
testNfsServerService() {
  local test="testNfsServerService"
  local service_name="nfs-server.service"
  echo "$test:Start"

  # is-enabled returns:
  # 'masked' if the service is masked.
  # empty string if the service is not installed.
  # 'not-found' if the unit files are not present. Encountered with Ubuntu 24.04
  echo "$test: Checking that $service_name is masked"
  local is_enabled=
  is_enabled=$(systemctl is-enabled $service_name 2>/dev/null)
  echo "$test: logging ${is_enabled} here"
  if [[ "${is_enabled}" == "masked" ]]; then
    echo "$test: $service_name is correctly masked"
  elif [[ "${is_enabled}" == "" || "${is_enabled}" == "not-found" ]]; then
    echo "$test: $service_name is not installed, which is fine"
  else
    err $test "$service_name is not masked"
  fi

  echo "$test:Finish"
}

# Tests that the pam.d settings are set correctly, per the function
# addFailLockDir in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh.
testPamDSettings() {
  local os_sku="${1}"
  local os_version="${2}"
  local test="testPamDSettings"
  local settings_file=/etc/security/faillock.conf
  echo "$test:Start"

  # We only want to run this test on Mariner/AzureLinux
  # So if it's anything else, report that we're skipping the test and bail.
  if [[ "${os_sku}" != "CBLMariner" && "${os_sku}" != "AzureLinux" ]]; then
    echo "$test: Skipping test on ${os_sku} ${os_version}"
  else

    # Existence and format check. The man page https://www.man7.org/linux/man-pages/man5/faillock.conf.5.html
    # describes the following format for each line:
    #   Comments start with '#'.
    #   Blank lines are ignored.
    #   Lines are of in two forms:
    #       'setting = value', where settings are lower-case and include '_'
    #       'setting'
    #   Whitespace at beginning and end of line, along with around the '=' is ignored.
    testSettingFileFormat $test $settings_file '^(#|$)' '^[[:space:]]*$' '^[[:space:]]*[a-z_][[:space:]]*' '^[[:space:]]*[a-z_]+[[:space::]]*=[^[:cntrl:]]*$'

    # Look for the setting we specifically set in <repo-root>/parts/linux/cloud-init/artifacts/cis.sh
    # and ensure it's set to the values we expect.
    echo "$test: Checking specific settings in $settings_file"
    testSetting $test $settings_file 'dir' '^[[:space:]]*dir[[:space:]]*=' '=' '/var/log/faillock'
  fi

  echo "$test:Finish"
}

# Checks a single file or directory's permissions.
# Parameters:
#  test: The name of the test.
#  path: The path to check.
#  expected_perms: The expected permissions.
#  required: If 1, the path must exist. If 0, the path is optional.
function checkPathPermissions() {
  local test="$1"
  local path="$2"
  local expected_perms="$3"
  local required="$4"

  echo "$test: Checking permissions for '$path'"
  if [ ! -e "$path" ]; then
    if [ "$required" -eq 1 ]; then
      err $test "Required path '$path' does not exist"
    else
      echo "$test: Optional path '$path' does not exist"
    fi
  else
    local actual_perms=
    actual_perms=$(stat -c %a $path)
    if [ "$actual_perms" != "$expected_perms" ]; then
      err $test "Path '$path' has permissions $actual_perms; expected $expected_perms"
    else
      echo "$test: $path has correct permissions $actual_perms"
    fi
  fi
}

# Checks that a single file or directory does not exist.
# Parameters:
#  test: The name of the test.
#  path: The path to check.
function checkPathDoesNotExist() {
  local test="$1"
  local path="$2"

  echo "$test: Checking that '$path' does not exist"
  if [ -e "$path" ]; then
    err $test "Path '$path' exists"
  else
    echo "$test: $path correctly does not exist"
  fi
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
      err $test "Invalid line $line_num in $settings_file: '$line'"
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

string_replace() {
  echo ${1//\*/$2}
}

# Tests that the PAM configuration is functional and aligns with the expected configuration.
testPam() {
  local os_sku="${1}"
  local os_version="${2}"
  local test="testPam"
  local testdir="./AgentBaker/vhdbuilder/packer/test/pam"
  local retval=0
  echo "${test}:Start"

  # We only want to run this test on Mariner/AzureLinux
  # So if it's anything else, report that we're skipping the test and bail.
  if [[ "${os_sku}" != "CBLMariner" && "${os_sku}" != "AzureLinux" ]]; then
    echo "$test: Skipping test on ${os_sku} ${os_version}"
  else
    # cd to the directory of the script
    pushd ${testdir} || (err ${test} "Failed to cd to test directory ${testdir}"; return 1)
    # create the virtual environment
    python3 -m venv . || (err ${test} "Failed to create virtual environment"; return 1)
    # activate the virtual environment
    # shellcheck source=/dev/null
    source ./bin/activate
    # install the dependencies
    pip3 install --disable-pip-version-check -r requirements.txt || \
      (err ${test} "Failed to install dependencies"; return 1)
    # run the script
    # the pam tests are flaky as they require scraping the console
    # if there are test failures, --reruns 5 will rerun the failed tests up to 5 times
    output=$(pytest -v -s --reruns 5 test_pam.py)
    retval=$?
    # deactivate the virtual environment
    deactivate
    popd || (err ${test} "Failed to cd out of test dir"; return 1)

    if [ $retval -ne 0 ]; then
      err ${test} "$output"
      err ${test} "PAM configuration is not functional"
      retval=1
    else
      echo "${test}: PAM configuration is functionally correct"
    fi
  fi

  echo "${test}:Finish"
  return $retval
}

testContainerImagePrefetchScript() {
  local test="testContainerImagePrefetchScript"
  local container_image_prefetch_script_path="/opt/azure/containers/prefetch.sh"

  echo "$test: checking existence of container image prefetch script at $container_image_prefetch_script_path"
  if [ ! -f "$container_image_prefetch_script_path" ]; then
    err "$test: container image prefetch script does not exist at $container_image_prefetch_script_path"
    return 1
  fi
  echo "$test: container image prefetch script exists at $container_image_prefetch_script_path"

  echo "$test: running container image prefetch script..."
  chmod +x $container_image_prefetch_script_path
  errs=$(/bin/bash $container_image_prefetch_script_path 2>&1 >/dev/null)
  code=$?
  if [ $code -ne 0 ]; then
    err "$test: container image prefetch script exited with code $code, stderr:\n$errs"
    return 1
  fi
  echo "$test: container image prefetch script ran successfully"

  return 0
}

testBccTools () {
  local test="BCCInstallTest"
  echo "$test: checking if BCC tools were successfully installed"
  for line in '  - bcc-tools' '  - libbcc-examples'; do
    if ! grep -F -x -e "$line" $VHD_LOGS_FILEPATH; then
      err "BCC tools were not successfully installed"
      return 1
    fi
  done
  echo "$test: BCC tools were successfully installed"
  return 0
}

testAKSNodeControllerBinary () {
  local test="testAKSNodeControllerBinary"
  local go_binary_path="/opt/azure/containers/aks-node-controller"

  echo "$test: checking existence of aks-node-controller go binary at $go_binary_path"
  if [ ! -f "$go_binary_path" ]; then
    err "$test: aks-node-controller go binary does not exist at $go_binary_path"
    return 1
  fi
  echo "$test: aks-node-controller go binary exists at $go_binary_path"
}

testAKSNodeControllerService() {
  local test="testNBCParserService"
  local service_name="aks-node-controller.service"
  echo "$test:Start"

  # is-enabled returns:
  # 'enabled' if the service is enabled.
  # empty string if the service is not installed.
  # 'not-found' if the unit files are not present. Encountered with Ubuntu 24.04
  echo "$test: Checking that $service_name is enabled"
  is_enabled=$(systemctl is-enabled $service_name 2>/dev/null)
  echo "$test: logging ${is_enabled} here"
  if [[ "${is_enabled}" == "enabled" ]]; then
    echo "$test: $service_name is correctly enabled"
  else
    err $test "$service_name is not enabled, instead in state $is_enabled"
  fi

  echo "$test:Finish"
}

testWasmRuntimesInstalled() {
  local test="testWasmRuntimesInstalled"
  local wasm_runtimes_path=${1}
  local shim_version=${2}

  echo "$test: checking existance of Spin Wasm Runtime in $wasm_runtimes_path"

  local shims_to_download=("spin" "slight")
  if [[ "${shim_version}" == "0.8.0" ]]; then
    shims_to_download+=("wws")
  fi

  binary_version="$(echo "${shim_version}" | tr . -)"
  for shim in "${shims_to_download[@]}"; do
    binary_path_pattern="${wasm_runtimes_path}/containerd-shim-${shim}-${binary_version}-*"
    if [ ! -f $binary_path_pattern ]; then
      output=$(ls -la /usr/local/bin)
      err "$test: Spin Wasm Runtime binary does not exist at $binary_path_pattern\n ls -la output:\n $output"
      return 1
    else
      echo "$test: Spin Wasm Runtime binary exists at $binary_path_pattern"
    fi
  done
}

testSpinKubeInstalled() {
  local test="testSpinKubeInstalled"
  local spinKube_runtimes_path=${1}
  local shim_version=${2}
  shim_version="v${shim_version}"
  binary_version="$(echo "${shim_version}" | tr . -)"

  # v0.15.1 does not have a version encoded in the binary name
  binary_path_pattern="${spinKube_runtimes_path}/containerd-shim-spin-v2"
  if [ ! -f $binary_path_pattern ]; then
    output=$(ls -la /usr/local/bin)
    err "$test: Spin Wasm Runtime binary does not exist at $binary_path_pattern\n ls -la output:\n $output"
    return 1
  else
    echo "$test: Spin Wasm Runtime binary exists at $binary_path_pattern"
  fi

  echo "$test: Test finished successfully."
  return 0
}

checkPerformanceData() {
  local test="checkPerformanceData"
  local performanceDataPath="/opt/azure/vhd-build-performance-data.json"

  echo "$test: Checking for existence of $performanceDataPath"
  if test -f "$performanceDataPath"; then
    err "$test: $performanceDataPath deletion was not successful."
    return 1
  else
    echo "File $performanceDataPath does not exist"
  fi
  echo "$test: Test finished successfully."
  return 0
}

#------------------------ Start of test code related to localdns ------------------------
testCoreDnsBinaryExtractedAndCached() {
  local test="CoreDnsBinaryExtractedAndCached"
  local os_version=$1
  # Ubuntu 18.04 and 20.04 ship with GLIBC 2.27 and 2.31, respectively.
  # CoreDNS binary is built with GLIBC 2.32+, which is not compatible with 18.04 and 20.04 OS versions.
  # Therefore, we skip the test for these OS versions here.
  # Validation in AKS RP will be done to ensure localdns is not enabled for these OS versions.
  if [[ ${os_version} == "18.04" || ${os_version} == "20.04" ]]; then
    # For Ubuntu 18.04 and 20.04, the coredns binary is located in /opt/azure/containers/localdns/binary/coredns
    echo "$test: CoreDNS is not supported on OS version: ${os_version}"
    return 0
  fi

  local localdnsBinaryDir="/opt/azure/containers/localdns/binary"
  local binaryPath="$localdnsBinaryDir/coredns"
  local coredns_image_list=($(ctr -n k8s.io images list -q | grep coredns))

  echo "$test: Checking for existence of coredns binary at ${binaryPath}"

  if [[ ! -f "${binaryPath}" ]]; then
    err "$test: coredns binary does not exist at ${binaryPath}"
    return 1
  fi

  if [[ ${#coredns_image_list[@]} -eq 0 ]]; then
    err "$test: No CoreDNS images found in the local container images"
    return 1
  fi

  # Extract available coredns image tags (v1.12.0-1 format) and sort them in descending order.
  local sorted_coredns_tags=($(for image in "${coredns_image_list[@]}"; do echo "${image##*:}"; done | sort -V -r))

  # Determine latest version (eg. v1.12.0-1).
  local latest_coredns_tag="${sorted_coredns_tags[0]}"
  # Extract major.minor.patch (removes -revision. eg - v1.12.0).
  local latest_vMajorMinorPatch="${latest_coredns_tag%-*}"

  local previous_coredns_tag=""
  # Iterate through the sorted list to find the next highest major-minor version.
  for tag in "${sorted_coredns_tags[@]}"; do
    # Extract major.minor.patch (eg - v1.12.0).
    local vMajorMinorPatch="${tag%-*}"
    if [[ "${vMajorMinorPatch}" != "${latest_vMajorMinorPatch}" ]]; then
      previous_coredns_tag="$tag"
      # Break the loop after the next highest major-minor version is found.
      break
    fi
  done

  if [[ -z "${previous_coredns_tag}" ]]; then
    echo "$test: Warning: Previous version not found, using the latest version: ${latest_coredns_tag}"
    previous_coredns_tag="$latest_coredns_tag"
  fi

  local expectedVersion="$previous_coredns_tag"
  local expectedVersionWithoutV="${expectedVersion#v}"
  echo "$test: Expected CoreDNS version (n-1 latest revision): ${expectedVersionWithoutV}"

  # Get the actual version from the extracted CoreDNS binary
  local actualVersion
  actualVersion=$("$binaryPath" --version | awk -F'-' '{print $2}')

  local actualVersionWithoutV="${actualVersion#v}"
  if [[ -z "${actualVersionWithoutV}" ]]; then
    err "$test: Failed to retrieve CoreDNS version from $binaryPath"
    return 1
  fi

  echo "$test: Verify extracted CoreDNS version: ${actualVersionWithoutV}"

  if [[ "${actualVersion%-*}" != "${expectedVersionWithoutV%-*}" ]]; then
    echo "$test: Extracted CoreDNS version: ${actualVersion} does not match expected version: ${expectedVersionWithoutV}"
    return 1
  fi

  echo "$test: Expected version: ${expectedVersionWithoutV} of coredns binary is extracted and cached at ${binaryPath}"
  return 0
}

testLocalDNSScriptsAndConfigs() {
  local test="testLocalDNSScriptsAndConfigs"
  
  declare -A localDNSfiles=(
    ["/opt/azure/containers/localdns/localdns.sh"]=755
    ["/etc/systemd/system/localdns.service"]=644
    ["/etc/systemd/system/localdns.slice"]=644
    ["/etc/systemd/system/localdns.service.d/delegate.conf"]=644
    ["/etc/systemd/resolved.conf.d/70-aks-dns.conf"]=644
    ["/etc/systemd/network/10-netplan-eth0.network.d/05-aks-keepconfig.conf"]=644
  )
  
  for file in "${!localDNSfiles[@]}"; do
    echo "$test: Checking existence of ${file}"
    if [ ! -f "${file}" ]; then
      err "$test: localDNSfile - ${file} not found"
      return 1
    fi
    
    echo "$test: Checking permissions of ${file}"
    permissions=$(stat -c "%a" "$file")
    if [ "$permissions" != "${localDNSfiles[$file]}" ]; then
      echo "$test: localDNSfile $file has incorrect permissions. Expected ${localDNSfiles[$file]}, got $permissions"
      return 1
    fi
  done
  
  echo "$test: All localDNSfile exist with correct permissions"
  return 0
}
#------------------------ End of test code related to localdns ------------------------

# As we call these tests, we need to bear in mind how the test results are processed by the
# the caller in run-tests.sh. That code uses az vm run-command invoke to run this script
# on a VM. It then looks at stderr to see if any errors were reported. Notably it doesn't
# look the exit code of this script -- in fact, it can't due to a limitation in the
# run-command invoke command. So we need to be careful to report errors to stderr
#
# We should also avoid early exit from the test run -- like if a command fails with
# an exit rather than a return -- because that prevents other tests from running.
# To repro the test results on the exact VM, we can set VHD_DEBUG="True" in the azure pipeline env variables.
# This will keep the VM alive after the tests are run and we can SSH/Bastion into the VM to run the test manually.
# Therefore, for example, you can run "sudo bash /var/lib/waagent/run-command/download/0/script.sh" to run the tests manually.
checkPerformanceData
testBccTools
testVHDBuildLogsExist
testCriticalTools
testPackagesInstalled $CONTAINER_RUNTIME
testImagesPulled $CONTAINER_RUNTIME "$(cat $COMPONENTS_FILEPATH)"
testChrony $OS_SKU
testAuditDNotPresent
testFips $OS_VERSION $ENABLE_FIPS
testCloudInit $OS_SKU
# Commenting out testImagesRetagged because at present it fails, but writes errors to stdout
# which means the test failures haven't been caught. It also calles exit 1 on a failure,
# which means the rest of the tests aren't being run.
# See https://msazure.visualstudio.com/CloudNativeCompute/_backlogs/backlog/Node%20Lifecycle/Features/?workitem=24246232
# testImagesRetagged $CONTAINER_RUNTIME
testCustomCAScriptExecutable
testCustomCATimerNotStarted
testCustomCATrustNodeCAWatcherRetagged
testLoginDefs
testUserAdd
testNetworkSettings
testCronPermissions $IMG_SKU
testCoreDumpSettings
testNfsServerService
testPamDSettings $OS_SKU $OS_VERSION
testPam $OS_SKU $OS_VERSION
testUmaskSettings
testContainerImagePrefetchScript
testAKSNodeControllerBinary
testAKSNodeControllerService
testLtsKernel $OS_VERSION $OS_SKU $ENABLE_FIPS
testCoreDnsBinaryExtractedAndCached $OS_VERSION
testLocalDNSScriptsAndConfigs