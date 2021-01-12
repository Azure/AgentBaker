#!/bin/bash
set -eux

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
  echo "Starting testFilesDownloaded"
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

  ls
  ls -R "/opt"
  ls -R "/usr/local/bin"

  PARAMETERS=$(echo "${PARAMETERS}" | jq . --monochrome-output --compact-output)
  emptyFiles=()
  missingPaths=()
  while IFS='' read -r param || [[ -n "${param}" ]]; do
    downloadURL=$(echo "${param}" | jq .downloadURL -r)
    downloadLocation=$(echo "${param}" | jq .downloadLocation -r)
    versions=$(echo "${param}" | jq .versions -r)

    if [ ! -d downloadLocation ]; then
      err "Directory ${downloadLocation} does not exist"
      missingPaths+=("$downloadLocation")
      continue
    fi

    for version in ${versions}; do
      downloadURL=$(string_replace $downloadURL version version)
      fileName=${downloadURL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
      dest="$downloadLocation/${fileName}"

      if [ ! -s dest ]; then
        err "File ${dest} does not exist"
        emptyFiles+=("$dest")
        continue
      fi
    done

    echo "---"
  done < <(echo "${PARAMETERS}")

  if ((${#emptyFiles[@]} > 0)) || ((${#missingPaths[@]} > 0)); then
    err "cache files base paths $missingPaths or(and) cached files $emptyFiles do not exist"
  fi
}

testImagesPulled() {
  echo "Starting testImagesPulled"
  containerRuntime=$1

  if [ $containerRuntime == 'containerd' ]; then
    pulledImages=$(ctr -n k8s.io -q)
  elif [ $containerRuntime == 'docker' ]; then
    pulledImages=$(docker images --format "{{.Repository}}:{{.Tag}}")
  else
    err "unsupported container runtime $containerRuntime"
    return
  fi

  imagesNotPulled=()

  containerImageObjects=$(jq -r ".[]" container-images.json | jq . --monochrome-output --compact-output)
  for containerImageObject in $containerImageObjects; do
    downloadURL=$(echo "${containerImageObject}" | jq .downloadURL -r)
    versions=$(echo "${containerImageObject}" | jq .versions -r)

    for version in ${versions}; do
      downloadURL=$(string_replace $downloadURL $version)

      if [[ $pulledImages =~ $downloadURL ]]; then
        echo "Image ${downloadURL} has been pulled Successfully"
      else
        err "Image ${downloadURL} has NOT been pulled"
        imagesNotPulled+=("$downloadURL")
      fi
    done

    echo "---"
  done
  if ((${#imagesNotPulled[@]} > 0)); then
    err "Some images were not successfully pulled \n $imagesNotPulled"
  fi
}

err(){
    echo "Error: $*" >>/dev/stderr
}

testFilesDownloaded
testImagesPulled $1
