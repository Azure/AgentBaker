#!/bin/bash

#cni plugins +
#azure vnet cni +
#bpftrace +
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
### installSGX=${SGX_DEVICE_PLUGIN_INSTALL:-"False"} needs condition -

testFilesDownloaded() {
  test="testFilesDownloaded"
  echo "$test:Start"
  filesToDownload=$1

  filesToDownload=$(echo $filesToDownload | jq -r ".[]" | jq . --monochrome-output --compact-output)
  emptyFiles=()
  missingPaths=()
  for fileToDownload in ${filesToDownload[*]}; do
    fileName=$(echo "${fileToDownload}" | jq .fileName -r)
    downloadLocation=$(echo "${fileToDownload}" | jq .downloadLocation -r)
    versions=$(echo "${fileToDownload}" | jq .versions -r | jq -r ".[]")

    if [ ! -d $downloadLocation ]; then
      err $test "Directory ${downloadLocation} does not exist"
      missingPaths+=("$downloadLocation")
      continue
    fi

    for version in ${versions}; do
      file_Name=$(string_replace $fileName $version)
      dest="$downloadLocation/${file_Name}"

      if [ ! -s $dest ]; then
        err $test "File ${dest} does not exist"
        emptyFiles+=("$dest")
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
  imagesToBePulled=$2
  echo '------------------- containerRuntime--------------------'
  echo "$containerRuntime"
  if [ $containerRuntime == 'containerd' ]; then
    pulledImages=$(ctr -n k8s.io -q)
  elif [ $containerRuntime == 'docker' ]; then
    pulledImages=$(docker images --format "{{.Repository}}:{{.Tag}}")
  else
    err $test "unsupported container runtime $containerRuntime"
    return
  fi

  imagesNotPulled=()

  imagesToBePulled=$(echo $imagesToBePulled | jq -r ".[]" | jq . --monochrome-output --compact-output)
  for imageToBePulled in ${imagesToBePulled[*]}; do
    downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
    versions=$(echo "${imageToBePulled}" | jq .versions -r | jq -r ".[]")

    for version in ${versions}; do
      download_URL=$(string_replace $downloadURL $version)

      if [[ $pulledImages =~ $downloadURL ]]; then
        echo "Image ${download_URL} has been pulled Successfully"
      else
        err $test "Image ${download_URL} has NOT been pulled"
        imagesNotPulled+=("$download_URL")
      fi
    done

    echo "---"
  done
  echo "$test:Finish"
}

err() {
  echo "$1:Error: $2" >>/dev/stderr
}

string_replace() {
  echo $1 | sed "s/\*/$2/"
}

filesToDownload='
[
{
  "fileName":"cni-plugins-amd64-v*.tgz",
  "downloadLocation":"/opt/cni/downloads",
  "versions": ["0.7.6","0.7.5","0.7.1"]
},
{
  "fileName":"cni-plugins-linux-amd64-v*.tgz",
  "downloadLocation":"/opt/cni/downloads",
  "versions": ["0.8.6"]
},
{
  "fileName":"azure-vnet-cni-linux-amd64-v*.tgz",
  "downloadLocation":"/opt/cni/downloads",
  "versions":["1.2.0_hotfix","1.2.0","1.1.8"]
},
{
  "fileName":"v*/bpftrace-tools.tar",
  "downloadLocation":"/opt/bpftrace/downloads",
  "versions": ["0.9.4"]
}
]
'
imagesToBePulled='
[
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/kubernetes-dashboard:v*",
    "versions": ["1.10.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/dashboard:v*",
    "versions": ["2.0.0-beta8","2.0.0-rc3","2.0.0-rc7","2.0.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/metrics-scraper:v*",
    "versions": ["1.0.2","1.0.3","1.0.4"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/exechealthz:*",
    "versions": ["1.2"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*",
    "versions": ["1.8.5","1.8.4","1.8.1","1.7"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/metrics-server:v*",
    "versions": ["0.3.6","0.3.5"]
  },
  {
    "downloadURL": "mcr.microsoft.com/k8s/core/pause:*",
    "versions": ["1.2.0"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/pause:*",
    "versions": ["1.2.0","1.3.1","1.4.0"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/coredns:*",
    "versions": ["1.6.6","1.6.5","1.5.0","1.3.1","1.2.6"]
  },
  {
    "downloadURL": "mcr.microsoft.com/containernetworking/networkmonitor:v*",
    "versions": ["1.1.8","0.0.7","0.0.6"]
  },
  {
    "downloadURL": "mcr.microsoft.com/containernetworking/azure-npm:v*",
    "versions": ["1.2.1","1.1.8","1.1.7","1.1.5","1.1.4"]
  },
  {
    "downloadURL": "mcr.microsoft.com/containernetworking/azure-vnet-telemetry:v*",
    "versions": ["1.0.30"]
  },
  {
    "downloadURL": "mcr.microsoft.com/aks/acc/sgx-device-plugin:*",
    "versions": ["1.0"]
  },
  {
    "downloadURL": "mcr.microsoft.com/aks/hcp/hcp-tunnel-front:v*",
    "versions": ["1.9.2-v3.0.18","1.9.2-v3.0.19","1.9.2-v3.0.20"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/apiserver-network-proxy/agent:v*",
    "versions": ["0.0.13"]
  },
  {
    "downloadURL": "mcr.microsoft.com/aks/hcp/kube-svc-redirect:v*",
    "versions": ["1.0.7"]
  },
  {
    "downloadURL": "mcr.microsoft.com/azuremonitor/containerinsights/ciprod:*",
    "versions": ["ciprod10052020","ciprod10272020","ciprod11092020"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/calico/cni:v*",
    "versions": ["3.8.9","3.8.9.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/calico/node:v*",
    "versions": ["3.8.9","3.8.9.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/calico/typha:v*",
    "versions": ["3.8.9","3.8.9.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/calico/pod2daemon-flexvol:v*",
    "versions": ["3.8.9","3.8.9.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/autoscaler/cluster-proportional-autoscaler:*",
    "versions": ["1.3.0_v0.0.5","1.7.1","1.7.1-hotfix.20200403"]
  },
  {
    "downloadURL": "mcr.microsoft.com/k8s/flexvolume/blobfuse-flexvolume:*",
    "versions": ["1.0.15"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/ip-masq-agent:v*",
    "versions": ["2.5.0.2","2.5.0.3"]
  },
  {
    "downloadURL": "mcr.microsoft.com/k8s/kms/keyvault:v*",
    "versions": ["0.0.9"]
  },
  {
    "downloadURL": "mcr.microsoft.com/k8s/csi/azuredisk-csi:v*",
    "versions": ["0.7.0","0.9.0"]
  },
  {
    "downloadURL": "mcr.microsoft.com/k8s/csi/azurefile-csi:v*",
    "versions": ["0.7.0","0.9.0"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v*",
    "versions": ["1.1.0"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v*",
    "versions": ["1.2.0","2.0.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v*",
    "versions": ["2.0.1","3.1.0","3.1.1","3.1.3"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/external-dns:v*",
    "versions": ["0.6.0-hotfix-20200228"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/defaultbackend:*",
    "versions": ["1.4"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/kubernetes/ingress/nginx-ingress-controller:*",
    "versions": ["0.19.0"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/virtual-kubelet/virtual-kubelet:*",
    "versions": ["1.2.1.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:*",
    "versions": ["prod_20200901.1","prod_20200923.1","prod_20201015.1"]
  },
  {
    "downloadURL": "mcr.microsoft.com/azure-policy/policy-kubernetes-webhook:*",
    "versions": ["prod_20200505.3"]
  },
  {
    "downloadURL": "mcr.microsoft.com/azure-application-gateway/kubernetes-ingress:*",
    "versions": ["1.0.1-rc3"]
  },
  {
    "downloadURL": "mcr.microsoft.com/oss/azure/aad-pod-identity/nmi:v*",
    "versions": ["1.6.3","1.7.2"]
  }
]
'

testFilesDownloaded "$filesToDownload"
testImagesPulled docker "$imagesToBePulled"
