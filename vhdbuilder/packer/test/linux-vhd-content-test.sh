#!/bin/bash
git clone https://github.com/Azure/AgentBaker.git 2>/dev/null
source ./AgentBaker/parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh 2>/dev/null
COMPONENTS_FILEPATH=/opt/azure/components.json
KUBE_PROXY_IMAGES_FILEPATH=/opt/azure/kube-proxy-images.json
THIS_DIR="$(cd "$(dirname ${BASH_SOURCE[0]})" && pwd)"

testFilesDownloaded() {
  test="testFilesDownloaded"
  containerRuntime=$1
  echo "$test:Start"
  filesToDownload=$(jq .DownloadFiles[] --monochrome-output --compact-output < $COMPONENTS_FILEPATH)

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
      # -L since some urls are redirects (i.e github)
      fileSizeInRepo=$(curl -sLI $downloadURL | grep -i Content-Length | tail -n1 | awk '{print $2}' | tr -d '\r')
      fileSizeDownloaded=$(wc -c $dest | awk '{print $1}' | tr -d '\r')
      if [[ "$fileSizeInRepo" != "$fileSizeDownloaded" ]]; then
        err $test "File size of ${dest} from ${downloadURL} is invalid. Expected file size: ${fileSizeInRepo} - downlaoded file size: ${fileSizeDownloaded}"
        continue
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
    versions=$(echo "${imageToBePulled}" | jq .versions -r | jq -r ".[]")
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
  status=$(systemctl show -p SubState --value chrony)
  if [ $status == 'running' ]; then
    echo $test "chrony is running, as expected"
  else
    err $test "chrony is not running with status ${status}"
  fi

  #test if chrony corrects time
  initialDate=$(date +%s)
  date --set "27 Feb 2021"
  for i in $(seq 1 10); do
    newDate=$(date +%s)
    if (( $newDate > $initialDate)); then
      echo "chrony readjusted the system time correctly"
      break
    fi
    sleep 10
    echo "${i}: retrying: check if chrony modified the time"
  done
  if (($i == 10)); then
    err $test "chrony failed to readjust the system time"
  fi
  echo "$test:Finish"
}

testFips() {
  test="testFips"
  echo "$test:Start"
  os_version=$1
  enable_fips=$2

  if [[ ${os_version} == "18.04" && ${enable_fips,,} == "true" ]]; then
    kernel=$(uname -r)
    if [[ -f /proc/sys/crypto/fips_enabled ]]; then
        echo "FIPS is enabled."
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
  k8sVersions="
  1.18.17-hotfix.20210322
  1.19.11-hotfix.20210823
  1.19.13-hotfix.20210830
  1.20.7-hotfix.20210816
  1.20.9-hotfix.20210830
  1.21.1-hotfix.20210827
  1.21.2-hotfix.20210830
  "
  for patchedK8sVersion in ${k8sVersions}; do
    # Only need to store k8s components >= 1.19 for containerd VHDs
    if (($(echo ${patchedK8sVersion} | cut -d"." -f2) < 19)) && [[ ${containerRuntime} == "containerd" ]]; then
      continue
    fi
    # strip the last .1 as that is for base image patch for hyperkube
    if grep -iq hotfix <<< ${patchedK8sVersion}; then
      # shellcheck disable=SC2006
      patchedK8sVersion=`echo ${patchedK8sVersion} | cut -d"." -f1,2,3,4`;
    else
      patchedK8sVersion=`echo ${patchedK8sVersion} | cut -d"." -f1,2,3`;
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
  dockerKubeProxyImages=$(jq .dockerKubeProxyImages < ${KUBE_PROXY_IMAGES_FILEPATH})
  containerdKubeProxyImages=$(jq .containerdKubeProxyImages < ${KUBE_PROXY_IMAGES_FILEPATH})

  if [ $containerRuntime == 'containerd' ]; then
    testImagesPulled containerd "$containerdKubeProxyImages"
  elif [ $containerRuntime == 'docker' ]; then
    testImagesPulled docker "$dockerKubeProxyImages"
  else
    err $test "unsupported container runtime $containerRuntime"
    return
  fi
  echo "$test:Finish"
}

err() {
  echo "$1:Error: $2" >>/dev/stderr
}

string_replace() {
  echo ${1//\*/$2}
}

testFilesDownloaded $1
testImagesPulled $1 "$(cat $COMPONENTS_FILEPATH)"
testChrony
testAuditDNotPresent
testFips $2 $3
testKubeBinariesPresent $1
testKubeProxyImagesPulled $1
testImagesRetagged $1
