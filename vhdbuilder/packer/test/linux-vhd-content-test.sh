#!/bin/bash

#cni plugins +
#azure vnet cni +
#installDeps -
#overrideNetworkConfig -
#disableSystemdTimesyncdAndEnableNTP -
#installImg +
# pullImage
### NVIDIA_DEVICE_PLUGIN_VERSION -
#GPU device plugin  -
### NGINX_VERSIONS -
### for PATCHED_KUBERNETES_VERSION in ${K8S_VERSIONS}; do -
### for KUBERNETES_VERSION in ${PATCHED_HYPERKUBE_IMAGES}; do
### for ADDON_IMAGE in ${ADDON_IMAGES}; do # easy to convert

testFilesDownloaded() {
  test="testFilesDownloaded"
  echo "$test:Start"
  PARAMETERS='{
                "downloadURL":"https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v*.tgz",
                "downloadLocation":"/opt/cni/downloads",
                "versions":"0.7.6 0.7.5 0.7.1"
              }
              {
                "downloadURL":"https://acs-mirror.azureedge.net/cni-plugins/v*/binaries/cni-plugins-linux-amd64-v*.tgz",
                "downloadLocation":"/opt/cni/downloads",
                "versions":"0.8.6"
              }
              {
                "downloadURL":"https://acs-mirror.azureedge.net/azure-cni/v*/binaries/azure-vnet-cni-linux-amd64-v*.tgz",
                "downloadLocation":"/opt/cni/downloads",
                "versions":"1.2.0_hotfix 1.2.0 1.1.8"
              }
              {
                "downloadURL":"https://acs-mirror.azureedge.net/img/img-linux-amd64-v*",
                "downloadLocation":"/usr/local/bin/img",
                "versions":"0.5.6"
              }'
  echo '------------------- printing ls --------------------'
  ls
  echo '------------------- printing ls -R "/opt"--------------------'
  ls -R "/opt"
  echo '------------------- printing ls -R "/usr/local/bin"--------------------'
  ls -R "/usr/local/bin"

  PARAMETERS=$(echo "${PARAMETERS}" | jq . --monochrome-output --compact-output)
  emptyFiles=()
  missingPaths=()
  while IFS='' read -r param || [[ -n "${param}" ]]; do
    downloadURL=$(echo "${param}" | jq .downloadURL -r)
    downloadLocation=$(echo "${param}" | jq .downloadLocation -r)
    versions=$(echo "${param}" | jq .versions -r)

    if [ ! -d $downloadLocation ]; then
      err $test "Directory ${downloadLocation} does not exist"
      missingPaths+=("$downloadLocation")
      continue
    fi

    for version in ${versions}; do
      downloadURL=$(string_replace $downloadURL version version)
      fileName=${downloadURL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
      dest="$downloadLocation/${fileName}"

      if [ ! -s $dest ]; then
        err $test "File ${dest} does not exist"
        emptyFiles+=("$dest")
        continue
      fi
    done

    echo "---"
  done < <(echo "${PARAMETERS}")

  if ((${#emptyFiles[@]} > 0)) || ((${#missingPaths[@]} > 0)); then
    err $test "cache files base paths $missingPaths or(and) cached files $emptyFiles do not exist"
  fi
  echo "$test:Finish"
}

testImagesPulled() {
  test="testImagesPulled"
  echo "$test:Start"
  containerRuntime=$1
  currentDirectory=$2

  if [ $containerRuntime == 'containerd' ]; then
    pulledImages=$(ctr -n k8s.io -q)
  elif [ $containerRuntime == 'docker' ]; then
    pulledImages=$(docker images --format "{{.Repository}}:{{.Tag}}")
  else
    err $test "unsupported container runtime $containerRuntime"
    return
  fi

  imagesNotPulled=()

  containerImageObjects=$(jq -r ".[]" $currentDirectory/container-images.json | jq . --monochrome-output --compact-output)
  for containerImageObject in $containerImageObjects; do
    downloadURL=$(echo "${containerImageObject}" | jq .downloadURL -r)
    versions=$(echo "${containerImageObject}" | jq .versions -r)

    for version in ${versions}; do
      downloadURL=$(string_replace $downloadURL $version)

      if [[ $pulledImages =~ $downloadURL ]]; then
        echo "Image ${downloadURL} has been pulled Successfully"
      else
        err $test "Image ${downloadURL} has NOT been pulled"
        imagesNotPulled+=("$downloadURL")
      fi
    done

    echo "---"
  done
  if ((${#imagesNotPulled[@]} > 0)); then
    err $test "Some images were not successfully pulled \n $imagesNotPulled"
  fi
  echo "$test:Finish"
}

err(){
    echo "$1 Error: $2" >>/dev/stderr
}

testFilesDownloaded
testImagesPulled $1 $2
