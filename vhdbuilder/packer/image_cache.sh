#!/bin/bash

pullSystemImages() {
    runtime=$1
    if [[ ${runtime} == "containerd" ]]; then
        cliTool="ctr"
    else 
        cliTool="docker"
    fi

    echo "${runtime} images pre-pulled:" >> ${VHD_LOGS_FILEPATH}

    DASHBOARD_VERSIONS="1.10.1"
    for DASHBOARD_VERSION in ${DASHBOARD_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/kubernetes-dashboard:v${DASHBOARD_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    NEW_DASHBOARD_VERSIONS="
    2.0.0-beta8
    2.0.0-rc3
    2.0.0-rc7
    2.0.1
    "
    for DASHBOARD_VERSION in ${NEW_DASHBOARD_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/dashboard:v${DASHBOARD_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    NEW_DASHBOARD_METRICS_SCRAPER_VERSIONS="
    1.0.2
    1.0.3
    1.0.4
    "
    for DASHBOARD_VERSION in ${NEW_DASHBOARD_METRICS_SCRAPER_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/metrics-scraper:v${DASHBOARD_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    EXECHEALTHZ_VERSIONS="1.2"
    for EXECHEALTHZ_VERSION in ${EXECHEALTHZ_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/exechealthz:${EXECHEALTHZ_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    ADDON_RESIZER_VERSIONS="
    1.8.5
    1.8.4
    1.8.1
    1.7
    "
    for ADDON_RESIZER_VERSION in ${ADDON_RESIZER_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:${ADDON_RESIZER_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    METRICS_SERVER_VERSIONS="
    0.3.6
    0.3.5
    "
    for METRICS_SERVER_VERSION in ${METRICS_SERVER_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/metrics-server:v${METRICS_SERVER_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    LEGACY_MCR_PAUSE_VERSIONS="1.2.0"
    for PAUSE_VERSION in ${LEGACY_MCR_PAUSE_VERSIONS}; do
        # Pull the arch independent MCR pause image which is built for Linux and Windows
        CONTAINER_IMAGE="mcr.microsoft.com/k8s/core/pause:${PAUSE_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    MCR_PAUSE_VERSIONS="
    1.2.0
    1.3.1
    1.4.0
    "
    for PAUSE_VERSION in ${MCR_PAUSE_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/pause:${PAUSE_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    CORE_DNS_VERSIONS="
    1.6.6
    1.6.5
    1.5.0
    1.3.1
    1.2.6
    "
    for CORE_DNS_VERSION in ${CORE_DNS_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/coredns:${CORE_DNS_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    AZURE_CNIIMAGEBASE="mcr.microsoft.com/containernetworking"
    AZURE_CNI_NETWORKMONITOR_VERSIONS="
    0.0.7
    0.0.6
    "
    for AZURE_CNI_NETWORKMONITOR_VERSION in ${AZURE_CNI_NETWORKMONITOR_VERSIONS}; do
        CONTAINER_IMAGE="${AZURE_CNIIMAGEBASE}/networkmonitor:v${AZURE_CNI_NETWORKMONITOR_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    AZURE_NPM_VERSIONS="
    1.1.7
    1.1.5
    1.1.4
    "
    for AZURE_NPM_VERSION in ${AZURE_NPM_VERSIONS}; do
        CONTAINER_IMAGE="${AZURE_CNIIMAGEBASE}/azure-npm:v${AZURE_NPM_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    AZURE_VNET_TELEMETRY_VERSIONS="
    1.0.30
    "
    for AZURE_VNET_TELEMETRY_VERSION in ${AZURE_VNET_TELEMETRY_VERSIONS}; do
        CONTAINER_IMAGE="${AZURE_CNIIMAGEBASE}/azure-vnet-telemetry:v${AZURE_VNET_TELEMETRY_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    NVIDIA_DEVICE_PLUGIN_VERSIONS="
    1.11
    1.10
    "
    for NVIDIA_DEVICE_PLUGIN_VERSION in ${NVIDIA_DEVICE_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="nvidia/k8s-device-plugin:${NVIDIA_DEVICE_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    # GPU device plugin
    if grep -q "fullgpu" <<< "$FEATURE_FLAGS" && grep -q "gpudaemon" <<< "$FEATURE_FLAGS"; then
        kubeletDevicePluginPath="/var/lib/kubelet/device-plugins"
        DEST="/usr/local/nvidia/bin"
        if [[ -d "${DEST}" ]]; then
            echo "gpu plugins already installed"
        else
            mkdir -p $kubeletDevicePluginPath
            echo "  - $kubeletDevicePluginPath" >> ${VHD_LOGS_FILEPATH}

            mkdir -p $DEST
            docker run --rm --entrypoint "" -v $DEST:$DEST "nvidia/k8s-device-plugin:1.11" /bin/bash -c "cp /usr/bin/nvidia-device-plugin $DEST" || exit 1
            chmod a+x $DEST/nvidia-device-plugin
            echo "  - extracted nvidia-device-plugin..." >> ${VHD_LOGS_FILEPATH}
            ls -ltr $DEST >> ${VHD_LOGS_FILEPATH}

            systemctlEnableAndStart nvidia-device-plugin || exit 1
        fi
    fi

    installSGX=${SGX_DEVICE_PLUGIN_INSTALL:-"False"}
    if [[ ${installSGX} == "True" ]]; then
        SGX_DEVICE_PLUGIN_VERSIONS="1.0"
        for SGX_DEVICE_PLUGIN_VERSION in ${SGX_DEVICE_PLUGIN_VERSIONS}; do
            CONTAINER_IMAGE="mcr.microsoft.com/aks/acc/sgx-device-plugin:${SGX_DEVICE_PLUGIN_VERSION}"
            pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
            echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
        done
    fi

    TUNNELFRONT_VERSIONS="
    v1.9.2-v3.0.17
    v1.9.2-v3.0.18
    v1.9.2-v4.0.15
    v1.9.2-v4.0.16
    "
    for TUNNELFRONT_VERSION in ${TUNNELFRONT_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/hcp/hcp-tunnel-front:${TUNNELFRONT_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    # 1.0.10 is for the ipv6 fix
    # 1.0.11 is for the cve fix
    OPENVPN_VERSIONS="
    1.0.8
    1.0.10
    1.0.11
    "
    for OPENVPN_VERSION in ${OPENVPN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/hcp/tunnel-openvpn:${OPENVPN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    KUBE_SVC_REDIRECT_VERSIONS="1.0.7"
    for KUBE_SVC_REDIRECT_VERSION in ${KUBE_SVC_REDIRECT_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/aks/hcp/kube-svc-redirect:v${KUBE_SVC_REDIRECT_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    # oms agent used by AKS
    # keeping last released image (ciprod07152020 - hotfix) and current to be released image (ciprod08072020)
    OMS_AGENT_IMAGES="ciprod07152020 ciprod08072020"
    for OMS_AGENT_IMAGE in ${OMS_AGENT_IMAGES}; do
        CONTAINER_IMAGE="mcr.microsoft.com/azuremonitor/containerinsights/ciprod:${OMS_AGENT_IMAGE}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    # calico images used by AKS
    CALICO_CNI_IMAGES="
    v3.5.0
    v3.8.0
    "
    for CALICO_CNI_IMAGE in ${CALICO_CNI_IMAGES}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/cni:${CALICO_CNI_IMAGE}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    CALICO_NODE_IMAGES="
    v3.5.0
    v3.8.0
    "
    for CALICO_NODE_IMAGE in ${CALICO_NODE_IMAGES}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/node:${CALICO_NODE_IMAGE}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    CALICO_TYPHA_IMAGES="
    v3.5.0
    v3.8.0
    "
    for CALICO_TYPHA_IMAGE in ${CALICO_TYPHA_IMAGES}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/typha:${CALICO_TYPHA_IMAGE}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    CALICO_POD2DAEMON_IMAGES="v3.8.0"
    for CALICO_POD2DAEMON_IMAGE in ${CALICO_POD2DAEMON_IMAGES}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/calico/pod2daemon-flexvol:${CALICO_POD2DAEMON_IMAGE}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    # Cluster Proportional Autoscaler
    CPA_IMAGES="
    1.3.0_v0.0.5
    1.7.1
    1.7.1-hotfix.20200403
    "
    for CPA_IMAGE in ${CPA_IMAGES}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/autoscaler/cluster-proportional-autoscaler:${CPA_IMAGE}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    KV_FLEXVOLUME_VERSIONS="0.0.13"
    for KV_FLEXVOLUME_VERSION in ${KV_FLEXVOLUME_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/k8s/flexvolume/keyvault-flexvolume:v${KV_FLEXVOLUME_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    BLOBFUSE_FLEXVOLUME_VERSIONS="1.0.13"
    for BLOBFUSE_FLEXVOLUME_VERSION in ${BLOBFUSE_FLEXVOLUME_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/k8s/flexvolume/blobfuse-flexvolume:${BLOBFUSE_FLEXVOLUME_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    # this is the patched images which AKS are using.
    AKS_IP_MASQ_AGENT_VERSIONS="
    2.5.0
    2.5.0.1
    "
    for IP_MASQ_AGENT_VERSION in ${AKS_IP_MASQ_AGENT_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes/ip-masq-agent:v${IP_MASQ_AGENT_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    NGINX_VERSIONS="1.13.12-alpine"
    for NGINX_VERSION in ${NGINX_VERSIONS}; do
        CONTAINER_IMAGE="nginx:${NGINX_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    KMS_PLUGIN_VERSIONS="0.0.9"
    for KMS_PLUGIN_VERSION in ${KMS_PLUGIN_VERSIONS}; do
        CONTAINER_IMAGE="mcr.microsoft.com/k8s/kms/keyvault:v${KMS_PLUGIN_VERSION}"
        pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
        echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
}


pullAddonImages() {
    runtime=$1
    if [[ ${runtime} == "containerd" ]]; then
        cliTool="ctr"
    else 
        cliTool="docker"
    fi

    ADDON_IMAGES="
    mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v2.0.1
    mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v3.1.0-beta.12
    mcr.microsoft.com/oss/open-policy-agent/gatekeeper:v3.1.0
    mcr.microsoft.com/oss/kubernetes/external-dns:v0.6.0-hotfix-20200228
    mcr.microsoft.com/oss/kubernetes/defaultbackend:1.4
    mcr.microsoft.com/oss/kubernetes/ingress/nginx-ingress-controller:0.19.0
    mcr.microsoft.com/oss/virtual-kubelet/virtual-kubelet:1.2.1.1
    mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:prod_20200804.1
    mcr.microsoft.com/azure-policy/policy-kubernetes-addon-prod:prod_20200901.1
    mcr.microsoft.com/azure-policy/policy-kubernetes-webhook:prod_20200505.3
    mcr.microsoft.com/azure-application-gateway/kubernetes-ingress:1.0.1-rc3
    "
    for ADDON_IMAGE in ${ADDON_IMAGES}; do
    pullContainerImage ${cliTool} ${ADDON_IMAGE}
    echo "  - ${ADDON_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    AZUREDISK_CSI_VERSIONS="
    0.7.0
    0.9.0
    "
    for AZUREDISK_CSI_VERSION in ${AZUREDISK_CSI_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/csi/azuredisk-csi:v${AZUREDISK_CSI_VERSION}"
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    AZUREFILE_CSI_VERSIONS="
    0.7.0
    0.9.0
    "
    for AZUREFILE_CSI_VERSION in ${AZUREFILE_CSI_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/k8s/csi/azurefile-csi:v${AZUREFILE_CSI_VERSION}"
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    CSI_LIVENESSPROBE_VERSIONS="
    1.1.0
    "
    for CSI_LIVENESSPROBE_VERSION in ${CSI_LIVENESSPROBE_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v${CSI_LIVENESSPROBE_VERSION}"
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done

    CSI_NODE_DRIVER_REGISTRAR_VERSIONS="
    1.2.0
    "
    for CSI_NODE_DRIVER_REGISTRAR_VERSION in ${CSI_NODE_DRIVER_REGISTRAR_VERSIONS}; do
    CONTAINER_IMAGE="mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v${CSI_NODE_DRIVER_REGISTRAR_VERSION}"
    pullContainerImage ${cliTool} ${CONTAINER_IMAGE}
    echo "  - ${CONTAINER_IMAGE}" >> ${VHD_LOGS_FILEPATH}
    done
}