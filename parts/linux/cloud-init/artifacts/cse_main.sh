#!/bin/bash
# Timeout waiting for a file
ERR_FILE_WATCH_TIMEOUT=6
set -x

if [ -f /opt/azure/containers/provision.complete ]; then
    echo "Already ran to success exiting..."
    exit 0
fi

# Cleanup cache file to force fetch fresh instance metadata from IMDS
rm -f /opt/azure/containers/imds_instance_metadata_cache.json

for i in $(seq 1 120); do
    if [ -s "${CSE_HELPERS_FILEPATH}" ]; then
        grep -Fq '#HELPERSEOF' "${CSE_HELPERS_FILEPATH}" && break
    fi
    if [ $i -eq 120 ]; then
        exit $ERR_FILE_WATCH_TIMEOUT
    else
        sleep 1
    fi
done

# Check for provisioning script hotfixes from MAR before sourcing any scripts.
# This function is intentionally self-contained — it cannot depend on any scripts
# it might replace (provision_source.sh, provision_installs.sh, etc.).
#
# Each hotfix targets exactly ONE baked VHD version (because a script with the
# same filename can differ between VHD versions). There is one tag per version:
#   <version>-hotfix  (e.g., v0.20260201.0-hotfix)
# The node checks for an exact tag match against its baked version.
check_for_script_hotfix() {
    local version_file="/opt/azure/containers/.provisioning-scripts-version"
    local registry="${HOTFIX_REGISTRY:-mcr.microsoft.com}"
    local applied_marker="/opt/azure/containers/.hotfix-applied"

    # Determine SKU from OS
    local sku=""
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        case "${ID}-${VERSION_ID}" in
            ubuntu-22.04) sku="ubuntu-2204" ;;
            ubuntu-24.04) sku="ubuntu-2404" ;;
            mariner-2*)   sku="azurelinux-v2" ;;
            azurelinux-3*) sku="azurelinux-v3" ;;
            *) echo "check_for_script_hotfix: unknown SKU (${ID}-${VERSION_ID}), skipping"; return 0 ;;
        esac
    else
        echo "check_for_script_hotfix: /etc/os-release not found, skipping"
        return 0
    fi

    local repo="${registry}/aks/provisioning-scripts/${sku}"

    # Read baked version
    if [ ! -f "$version_file" ]; then
        echo "check_for_script_hotfix: no baked version stamp found, skipping"
        return 0
    fi
    local baked_version
    baked_version=$(cat "$version_file")

    # Check ORAS availability
    if ! command -v oras &>/dev/null; then
        echo "check_for_script_hotfix: oras not available, skipping"
        return 0
    fi

    # List available hotfix tags (anonymous pull from public MAR, no auth needed)
    local tags
    tags=$(timeout 30 oras repo tags "$repo" 2>/dev/null) || {
        echo "check_for_script_hotfix: failed to query MAR for hotfixes, using baked scripts"
        return 0
    }

    if [ -z "$tags" ]; then
        echo "check_for_script_hotfix: no hotfixes available, using baked scripts"
        return 0
    fi

    local staging_dir="/opt/azure/containers/.hotfix-staging"
    local hotfix_tag="${baked_version}-hotfix"
    local applied_marker="/opt/azure/containers/.hotfix-applied"

    # Check if our version's hotfix tag exists in the tag list
    if ! echo "$tags" | grep -qx "$hotfix_tag"; then
        echo "check_for_script_hotfix: no hotfix for baked version $baked_version, using baked scripts"
        return 0
    fi

    # Skip if already applied
    if [ -f "$applied_marker" ] && grep -q "$hotfix_tag" "$applied_marker"; then
        echo "check_for_script_hotfix: hotfix $hotfix_tag already applied, skipping"
        return 0
    fi

    echo "check_for_script_hotfix: hotfix $hotfix_tag found for baked version $baked_version"

    # Pull the full artifact
    mkdir -p "$staging_dir"
    if ! timeout 30 oras pull "${repo}:${hotfix_tag}" \
        -o "$staging_dir" 2>/dev/null; then
        echo "check_for_script_hotfix: failed to pull $hotfix_tag, using baked scripts"
        rm -rf "$staging_dir"
        return 0
    fi

    # Verify metadata exists
    local metadata="$staging_dir/hotfix-metadata.json"
    if [ ! -f "$metadata" ]; then
        echo "check_for_script_hotfix: no metadata in $hotfix_tag, skipping"
        rm -rf "$staging_dir"
        return 0
    fi

    # Verify the metadata's affectedVersion matches (defense in depth)
    local meta_version
    if command -v jq &>/dev/null; then
        meta_version=$(jq -r '.affectedVersion' "$metadata" 2>/dev/null)
    else
        meta_version=$(grep -o '"affectedVersion"[[:space:]]*:[[:space:]]*"[^"]*"' "$metadata" | head -1 | sed 's/.*"\([^"]*\)"$/\1/')
    fi

    if [ "$meta_version" != "$baked_version" ]; then
        echo "check_for_script_hotfix: metadata version mismatch ($meta_version != $baked_version), skipping"
        rm -rf "$staging_dir"
        return 0
    fi

    # Find and extract the tarball
    local tarball
    tarball=$(find "$staging_dir" -name "*.tar.gz" | head -1)
    if [ -n "$tarball" ]; then
        # Verify tarball checksum if metadata includes it
        local expected_sha
        if command -v jq &>/dev/null; then
            expected_sha=$(jq -r '.tarballSha256 // empty' "$metadata" 2>/dev/null)
        fi
        if [ -n "${expected_sha:-}" ]; then
            local actual_sha
            actual_sha=$(sha256sum "$tarball" | awk '{print $1}')
            if [ "$actual_sha" != "$expected_sha" ]; then
                echo "check_for_script_hotfix: checksum mismatch for $hotfix_tag, using baked scripts"
                rm -rf "$staging_dir"
                return 0
            fi
        fi

        tar -xzf "$tarball" -C / --no-same-owner 2>/dev/null && {
            echo "$hotfix_tag" >> "$applied_marker"
            echo "check_for_script_hotfix: applied hotfix $hotfix_tag"
        } || {
            echo "check_for_script_hotfix: failed to extract $hotfix_tag, continuing with baked scripts"
        }
    fi

    rm -rf "$staging_dir"

    return 0
}

check_for_script_hotfix || true

source "${CSE_HELPERS_FILEPATH}"
source "${CSE_DISTRO_HELPERS_FILEPATH}"

# Setup logs for upload to host
LOG_DIR=/var/log/azure/aks
mkdir -p ${LOG_DIR}
ln -s /var/log/azure/cluster-provision.log \
      /var/log/azure/cluster-provision-cse-output.log \
      /opt/azure/*.json \
      /opt/azure/cloud-init-files.paved \
      /opt/azure/vhd-install.complete \
      ${LOG_DIR}/

# Redact the necessary secrets from cloud-config.txt so we don't expose any sensitive information
# when cloud-config.txt gets included within log bundles
python3 /opt/azure/containers/provision_redact_cloud_config.py \
    --cloud-config-path /var/lib/cloud/instance/cloud-config.txt \
    --output-path ${LOG_DIR}/cloud-config.txt

echo $(date),$(hostname), startcustomscript>>/opt/m

source "${CSE_INSTALL_FILEPATH}"
source "${CSE_DISTRO_INSTALL_FILEPATH}"
source "${CSE_CONFIG_FILEPATH}"

get_ubuntu_release() {
    lsb_release -r -s 2>/dev/null || echo ""
}

# ====== BASE PREP: BASE IMAGE PREPARATION ======
# This stage prepares the base VHD image with all necessary components and configurations.
# IMPORTANT: This stage must NOT join the node to the cluster.
# After completion, this VHD can be used as a base image for creating new node pools.
# Users may add custom configurations or pull additional container images after this stage.
function basePrep {
    logs_to_events "AKS.CSE.aptmarkWALinuxAgent" aptmarkWALinuxAgent hold &

    logs_to_events "AKS.CSE.configureAdminUser" configureAdminUser

    UBUNTU_RELEASE=$(get_ubuntu_release)
    if [ "${UBUNTU_RELEASE}" = "16.04" ]; then
        apt-get -y autoremove chrony
        echo $?
        systemctl restart systemd-timesyncd
    fi

    # Eval proxy vars to ensure curl commands use proxy if configured.
    # e.g. PROXY_VARS=`export HTTPS_PROXY="https://proxy.example.com:8080"; export http_proxy="http://proxy.example.com:8080"; export NO_PROXY="127.0.0.1,localhost";`
    # Setting vars in etc environment (configureEtcEnvironment) won't take effect in current shell session.
    if [ -n "${PROXY_VARS}" ]; then
        eval $PROXY_VARS
    fi

    resolve_packages_source_url
    logs_to_events "AKS.CSE.setPackagesBaseURL" "echo $PACKAGE_DOWNLOAD_BASE_URL"


    logs_to_events "AKS.CSE.fetch_and_cache_imds_instance_metadata" fetch_and_cache_imds_instance_metadata
    # This function creates the /etc/kubernetes/azure.json file. It also creates the custom
    # cloud configuration file if running in a custom cloud environment.
    logs_to_events "AKS.CSE.configureAzureJson" configureAzureJson

    logs_to_events "AKS.CSE.ensureKubeCACert" ensureKubeCACert

    logs_to_events "AKS.CSE.installSecureTLSBootstrapClient" installSecureTLSBootstrapClient

    logs_to_events "AKS.CSE.configureSSHPubkeyAuth" configureSSHPubkeyAuth "${DISABLE_PUBKEY_AUTH}"


    if [ "${DISABLE_SSH}" = "true" ]; then
        disableSSH || exit $ERR_DISABLE_SSH
    fi

    # This involves using proxy, log the config before fetching packages
    echo "private egress proxy address is '${PRIVATE_EGRESS_PROXY_ADDRESS}'"
    # TODO update to use proxy

    if [ "${SHOULD_CONFIGURE_HTTP_PROXY}" = "true" ]; then
        if [ "${SHOULD_CONFIGURE_HTTP_PROXY_CA}" = "true" ]; then
            configureHTTPProxyCA || exit $ERR_UPDATE_CA_CERTS
        fi
        configureEtcEnvironment
    fi

    if [ "${SHOULD_CONFIGURE_CUSTOM_CA_TRUST}" = "true" ]; then
        logs_to_events "AKS.CSE.configureCustomCaCertificate" configureCustomCaCertificate || exit $ERR_UPDATE_CA_CERTS
    fi

    logs_to_events "AKS.CSE.setCPUArch" setCPUArch
    source /etc/os-release

    if [ "${ID}" != "mariner" ] && [ "${ID}" != "azurelinux" ]; then
        echo "Removing man-db auto-update flag file..."
        logs_to_events "AKS.CSE.removeManDbAutoUpdateFlagFile" removeManDbAutoUpdateFlagFile
    fi

    # oras login must be in front of configureKubeletAndKubectl and ensureKubelet
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        # Compute registry domain name for ORAS login
        registry_domain_name="${MCR_REPOSITORY_BASE:-mcr.microsoft.com}"
        registry_domain_name="${registry_domain_name%/}"
        if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
            registry_domain_name="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER%%/*}"
        fi

        logs_to_events "AKS.CSE.orasLogin.oras_login_with_kubelet_identity" oras_login_with_kubelet_identity "${registry_domain_name}" $USER_ASSIGNED_IDENTITY_ID $TENANT_ID || exit $?
    fi

    logs_to_events "AKS.CSE.disableSystemdResolved" disableSystemdResolved

    export -f getInstallModeAndCleanupContainerImages
    export -f should_skip_binary_cleanup

    SKIP_BINARY_CLEANUP=$(should_skip_binary_cleanup)
    # this needs better fix to separate logs and return value;
    FULL_INSTALL_REQUIRED=$(getInstallModeAndCleanupContainerImages "$SKIP_BINARY_CLEANUP" "$IS_VHD" | tail -1)
    if [ "$?" -ne 0 ]; then
        echo "Failed to get the install mode and cleanup container images"
        exit "$ERR_CLEANUP_CONTAINER_IMAGES"
    fi

    if [ "$OS" = "$UBUNTU_OS_NAME" ] && [ "$FULL_INSTALL_REQUIRED" = "true" ]; then
        logs_to_events "AKS.CSE.installDeps" installDeps
    else
        echo "Golden image; skipping dependencies installation"
    fi

    # Container runtime already installed on Azure Linux OS Guard
    if ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
        logs_to_events "AKS.CSE.installContainerRuntime" installContainerRuntime
    fi
    if [ "${TELEPORT_ENABLED}" = "true" ]; then
        logs_to_events "AKS.CSE.installTeleportdPlugin" installTeleportdPlugin
    fi

    setupCNIDirs

    # Network plugin already installed on Azure Linux OS Guard
    if ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
        logs_to_events "AKS.CSE.installNetworkPlugin" installNetworkPlugin
    fi

    # ShouldEnforceKubePMCInstall is a nodepool tag we curl from IMDS.
    # Added as a temporary workaround to test installing packages from PMC prior to 1.34.0 GA.
    # TODO: Remove tag and usages once 1.34.0 is GA.
    export -f should_enforce_kube_pmc_install
    SHOULD_ENFORCE_KUBE_PMC_INSTALL=$(should_enforce_kube_pmc_install)
    logs_to_events "AKS.CSE.configureKubeletAndKubectl" configureKubeletAndKubectl

    createKubeManifestDir

    if [ "${HAS_CUSTOM_SEARCH_DOMAIN}" = "true" ]; then
        "${CUSTOM_SEARCH_DOMAIN_FILEPATH}" > /opt/azure/containers/setup-custom-search-domain.log 2>&1 || exit $ERR_CUSTOM_SEARCH_DOMAINS_FAIL
    fi

    # If the kubelet.service drop-in directory is empty by the time installContainerRuntime is called, it may be removed
    # as a side-effect of having to go out and install an uncached version of containerd. Thus, we once again create
    # the kubelet.service drop-in directory here before creating any further drop-ins.
    mkdir -p "/etc/systemd/system/kubelet.service.d"

    logs_to_events "AKS.CSE.configureCNI" configureCNI

    # configure and enable dhcpv6 for dual stack feature
    if [ "${IPV6_DUAL_STACK_ENABLED}" = "true" ]; then
        logs_to_events "AKS.CSE.ensureDHCPv6" ensureDHCPv6
    fi

    # For systemd in Azure Linux, UseDomains= is by default disabled for security purposes. Enable this
    # configuration within Azure Linux AKS that operates on trusted networks to support hostname resolution
    if isMarinerOrAzureLinux "$OS"; then
        logs_to_events "AKS.CSE.configureSystemdUseDomains" configureSystemdUseDomains
    fi

    # containerd should not be configured until cni has been configured first
    logs_to_events "AKS.CSE.ensureContainerd" ensureContainerd

    if [ -n "${MESSAGE_OF_THE_DAY}" ]; then
        if isMarinerOrAzureLinux "$OS" && [ -f /etc/dnf/automatic.conf ]; then
          sed -i "s/emit_via = motd/emit_via = stdio/g" /etc/dnf/automatic.conf
        elif [ "$OS" = "$UBUNTU_OS_NAME" ] && [ -d "/etc/update-motd.d" ]; then
              aksCustomMotdUpdatePath=/etc/update-motd.d/99-aks-custom-motd
              touch "${aksCustomMotdUpdatePath}"
              chmod 0755 "${aksCustomMotdUpdatePath}"
              echo -e "#!/bin/bash\ncat /etc/motd" > "${aksCustomMotdUpdatePath}"
        fi
        echo "${MESSAGE_OF_THE_DAY}" | base64 -d > /etc/motd
    fi

    # must run before kubelet starts to avoid race in container status using wrong image
    # https://github.com/kubernetes/kubernetes/issues/51017
    # can remove when fixed
    if [ "${TARGET_CLOUD}" = "AzureChinaCloud" ]; then
        retagMCRImagesForChina
    fi

    if [ "${ENABLE_HOSTS_CONFIG_AGENT}" = "true" ]; then
        logs_to_events "AKS.CSE.configPrivateClusterHosts" configPrivateClusterHosts
    fi

    if [ "${SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE}" = "true" ]; then
        logs_to_events "AKS.CSE.configureTransparentHugePage" configureTransparentHugePage
    fi

    if [ "${SHOULD_CONFIG_SWAP_FILE}" = "true" ]; then
        logs_to_events "AKS.CSE.configureSwapFile" configureSwapFile
    fi

    if [ "${NEEDS_CGROUPV2}" = "true" ]; then
        tee "/etc/systemd/system/kubelet.service.d/10-cgroupv2.conf" > /dev/null <<EOF
[Service]
Environment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd"
EOF
    fi

    # gross, but the backticks make it very hard to do in Go
    # TODO: move entirely into vhd.
    # alternatively, can we verify this is safe with docker?
    # or just do it even if not because docker is out of support?
    mkdir -p /etc/containerd
    echo "${KUBENET_TEMPLATE}" | base64 -d > /etc/containerd/kubenet_template.conf

    # In k8s 1.27, the flag --container-runtime was removed.
    # We now have 2 drop-in's, one with the still valid flags that will be applied to all k8s versions,
    # the flags are --runtime-request-timeout, --container-runtime-endpoint, --runtime-cgroups
    # For k8s >= 1.27, the flag --container-runtime will not be passed.
    tee "/etc/systemd/system/kubelet.service.d/10-containerd-base-flag.conf" > /dev/null <<'EOF'
[Service]
Environment="KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=/system.slice/containerd.service"
EOF

    if ! semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.27.0"; then
        tee "/etc/systemd/system/kubelet.service.d/10-container-runtime-flag.conf" > /dev/null <<'EOF'
[Service]
Environment="KUBELET_CONTAINER_RUNTIME_FLAG=--container-runtime=remote"
EOF
    fi

    if [ "${HAS_KUBELET_DISK_TYPE}" = "true" ]; then
        tee "/etc/systemd/system/kubelet.service.d/10-bindmount.conf" > /dev/null <<EOF
[Unit]
Requires=bind-mount.service
After=bind-mount.service
EOF
    fi

    logs_to_events "AKS.CSE.ensureSysctl" ensureSysctl || exit $ERR_SYSCTL_RELOAD

    if [ "${SHOULD_CONFIG_CONTAINERD_ULIMITS}" = "true" ]; then
      logs_to_events "AKS.CSE.setContainerdUlimits" configureContainerdUlimits
    fi

    if [ "${ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE}" = "true" ]; then
        logs_to_events "AKS.CSE.ensureNoDupOnPromiscuBridge" ensureNoDupOnPromiscuBridge
    fi

    if ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
        if [ "$OS" = "$UBUNTU_OS_NAME" ] || isMarinerOrAzureLinux "$OS"; then
            logs_to_events "AKS.CSE.ubuntuSnapshotUpdate" ensureSnapshotUpdate
        fi
    fi

    if [ "$FULL_INSTALL_REQUIRED" = "true" ]; then
        if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
            # mitigation for bug https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1676635
            echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
            sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
        fi
    fi

    if [ "${ARTIFACT_STREAMING_ENABLED}" = "true" ]; then
        logs_to_events "AKS.CSE.ensureContainerd.ensureArtifactStreaming" ensureArtifactStreaming || exit $ERR_ARTIFACT_STREAMING_INSTALL
    fi

    # This is to enable localdns using scriptless.
    if [ "${SHOULD_ENABLE_LOCALDNS}" = "true" ]; then
        logs_to_events "AKS.CSE.enableLocalDNS" enableLocalDNS || exit $ERR_LOCALDNS_FAIL
    fi

    if [ "${ID}" != "mariner" ] && [ "${ID}" != "azurelinux" ]; then
        echo "Recreating man-db auto-update flag file and kicking off man-db update process at $(date)"
        createManDbAutoUpdateFlagFile
        /usr/bin/mandb && echo "man-db finished updates at $(date)" &
    fi
}

# ====== NODE PREP: CLUSTER INTEGRATION ======
# This stage performs cluster-specific operations and hardware configurations.
# After this stage the node should be fully integrated into the cluster.
# IMPORTANT: This stage should only run when actually joining a node to the cluster. This step should not be run when creating a VHD image
function nodePrep {
    logs_to_events "AKS.CSE.fetch_and_cache_imds_instance_metadata" fetch_and_cache_imds_instance_metadata
    # IMPORTANT NOTE: We do this here since this function can mutate kubelet flags and node labels,
    # which is used by configureK8s and other functions. Thus, we need to make sure flag and label content is correct beforehand.
    logs_to_events "AKS.CSE.configureKubeletServing" configureKubeletServing

    # This function first creates the systemd drop-in directory for kubelet.service.
    # Pay attention to ordering relative to other functions that create kubelet drop-ins.
    logs_to_events "AKS.CSE.configureK8s" configureK8s

    if [ "${ENABLE_SECURE_TLS_BOOTSTRAPPING}" = "true" ]; then
        # Depends on configureK8s, ensureKubeCACert, and installSecureTLSBootstrapClient
        logs_to_events "AKS.CSE.configureAndStartSecureTLSBootstrapping" configureAndStartSecureTLSBootstrapping
    fi

    if [ -n "${OUTBOUND_COMMAND}" ]; then
        if [ -n "${PROXY_VARS}" ]; then
            eval $PROXY_VARS
        fi
        retrycmd_if_failure 60 1 5 $OUTBOUND_COMMAND >> /var/log/azure/cluster-provision-cse-output.log 2>&1 || exit $ERR_OUTBOUND_CONN_FAIL;
    fi
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        # This file indicates the cluster doesn't have outbound connectivity and should be excluded in future external outbound checks
        touch /var/run/outbound-check-skipped # TODO(fseldow): remove this file in future when egress extension checks /opt/azure/outbound-check-skipped
        touch /opt/azure/outbound-check-skipped
    fi

    # Configure Azure network settings (udev rules for NIC configuration)
    logs_to_events "AKS.CSE.ensureAzureNetworkConfig" ensureAzureNetworkConfig

    # Determine if GPU driver installation should be skipped
    export -f should_skip_nvidia_drivers
    skip_nvidia_driver_install=$(should_skip_nvidia_drivers)

    if [ "$?" -ne 0 ]; then
        echo "Failed to determine if nvidia driver install should be skipped"
        exit $ERR_NVIDIA_DRIVER_INSTALL
    fi

    # By default, never reboot new nodes.
    REBOOTREQUIRED=false

    # Clean up GPU drivers if not a GPU node or if skipping driver install
    if [ "${GPU_NODE}" != "true" ] || [ "${skip_nvidia_driver_install}" = "true" ]; then
        logs_to_events "AKS.CSE.cleanUpGPUDrivers" cleanUpGPUDrivers
    fi

    # Install and configure GPU drivers if this is a GPU node
    if [ "${GPU_NODE}" = "true" ] && [ "${skip_nvidia_driver_install}" != "true" ]; then
        echo $(date),$(hostname), "Start configuring GPU drivers"

        # Install GPU drivers
        logs_to_events "AKS.CSE.ensureGPUDrivers" ensureGPUDrivers

        # Install fabric manager if needed
        if [ "${GPU_NEEDS_FABRIC_MANAGER}" = "true" ]; then
            # fabric manager trains nvlink connections between multi instance gpus.
            # it appears this is only necessary for systems with *multiple cards*.
            # i.e., an A100 can be partitioned a maximum of 7 ways.
            # An NC24ads_A100_v4 has one A100.
            # An ND96asr_v4 has eight A100, for a maximum of 56 partitions.
            # ND96 seems to require fabric manager *even when not using mig partitions*
            # while it fails to install on NC24.
            if isMarinerOrAzureLinux "$OS"; then
                logs_to_events "AKS.CSE.installNvidiaFabricManager" installNvidiaFabricManager
            fi
            # Start fabric manager service
            logs_to_events "AKS.CSE.nvidia-fabricmanager" "systemctlEnableAndStart nvidia-fabricmanager 30" || exit $ERR_GPU_DRIVERS_START_FAIL
        fi

        # Configure MIG partitions if needed
        # This will only be true for multi-instance capable VM sizes
        # for which the user has specified a partitioning profile.
        # it is valid to use mig-capable gpus without a partitioning profile.
        if [ "${MIG_NODE}" = "true" ]; then
            # A100 GPU has a bit in the physical card (infoROM) to enable mig mode.
            # Changing this bit in either direction requires a VM reboot on Azure (hypervisor/plaform stuff).
            # Commands such as `nvidia-smi --gpu-reset` may succeed,
            # while commands such as `nvidia-smi -q` will show mismatched current/pending mig mode.
            # this will not be required per nvidia for next gen H100.
            REBOOTREQUIRED=true

            # this service applies the partitioning scheme with nvidia-smi.
            # we should consider moving to mig-parted which is simpler/newer.
            # we couldn't because of old drivers but that has long been fixed.
            logs_to_events "AKS.CSE.ensureMigPartition" ensureMigPartition
        fi

        # Configure managed GPU experience (device-plugin, dcgm, dcgm-exporter)
        export -f should_enable_managed_gpu_experience
        ENABLE_MANAGED_GPU_BY_TAG=$(should_enable_managed_gpu_experience)
        if [ "$?" -ne 0 ]; then
            echo "failed to determine if managed GPU experience should be enabled by nodepool tags"
            exit $ERR_LOOKUP_ENABLE_MANAGED_GPU_EXPERIENCE_TAG
        fi

        # Combine NBC and tag-based settings
        if [ "${ENABLE_MANAGED_GPU_BY_TAG}" = "true" ] || [ "${ENABLE_MANAGED_GPU,,}" = "true" ]; then
            ENABLE_MANAGED_GPU_EXPERIENCE="true"
        fi

        logs_to_events "AKS.CSE.configureManagedGPUExperience" configureManagedGPUExperience || exit $ERR_ENABLE_MANAGED_GPU_EXPERIENCE

        echo $(date),$(hostname), "End configuring GPU drivers"
    fi

    # Install and configure AMD AMA (Supernova) drivers if this is an AMA node
    if isAmdAmaEnabledNode; then
        logs_to_events "AKS.CSE.setupAmdAma" setupAmdAma
    fi

    VALIDATION_ERR=0

    # TODO(djsly): Look at leveraging the `aks-check-network.sh` script for this validation instead of duplicating the logic here

    # Edge case scenarios:
    # high retry times to wait for new API server DNS record to replicate (e.g. stop and start cluster)
    # high timeout to address high latency for private dns server to forward request to Azure DNS
    # dns check will be done only if we use FQDN for API_SERVER_NAME
    API_SERVER_CONN_RETRIES=50
    # shellcheck disable=SC3010
    if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
        API_SERVER_CONN_RETRIES=100
    fi
    # shellcheck disable=SC3010
    if ! [[ ${API_SERVER_NAME} =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        API_SERVER_DNS_RETRY_TIMEOUT=300
        if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
           API_SERVER_DNS_RETRY_TIMEOUT=600
        fi
        if [ "${ENABLE_HOSTS_CONFIG_AGENT}" != "true" ]; then
            RES=$(logs_to_events "AKS.CSE.apiserverNslookup" "retrycmd_nslookup 1 15 ${API_SERVER_DNS_RETRY_TIMEOUT} ${API_SERVER_NAME}")
            STS=$?
        else
            STS=0
        fi
        if [ "$STS" -ne 0 ]; then
            time nslookup ${API_SERVER_NAME}
            # shellcheck disable=SC3010
            if [[ $RES == *"168.63.129.16"*  ]]; then
                VALIDATION_ERR=$ERR_K8S_API_SERVER_AZURE_DNS_LOOKUP_FAIL
            else
                VALIDATION_ERR=$ERR_K8S_API_SERVER_DNS_LOOKUP_FAIL
            fi
        else
            logs_to_events "AKS.CSE.apiserverCurl" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 curl -v --cacert /etc/kubernetes/certs/ca.crt https://${API_SERVER_NAME}:443" || time curl -v --cacert /etc/kubernetes/certs/ca.crt "https://${API_SERVER_NAME}:443" || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
        fi
    else
        # an IP address is provided for the API server, skip the DNS lookup
        # this is the scenario for APIServerVnetIntegration. Currently we need more time to wait for the API server to be ready when feature is in preview.
        # switching back from curl to netcat for VNETIntegration scenario in combination with HTTP Proxy due to curl 7.81.0 not supporting CIDRs(no_proxy)
        # Once curl is available at 7.86.0 or higher, move this check from netcat to curl
        API_SERVER_CONN_RETRIES=300
        logs_to_events "AKS.CSE.apiserverNC" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443" || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
    fi

    echo "API server connection check code: $VALIDATION_ERR"
    if [ "$VALIDATION_ERR" -ne 0 ]; then
        exit $VALIDATION_ERR
    fi

    logs_to_events "AKS.CSE.ensureKubelet" ensureKubelet

    if $REBOOTREQUIRED; then
        echo 'reboot required, rebooting node in 1 minute'
        /bin/bash -c "shutdown -r 1 &"
        if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
            # logs_to_events should not be run on & commands
            aptmarkWALinuxAgent unhold &
        fi
    else
        if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
            # logs_to_events should not be run on & commands
            if [ "${ENABLE_UNATTENDED_UPGRADES}" = "true" ]; then
                UU_CONFIG_DIR="/etc/apt/apt.conf.d/99periodic"
                mkdir -p "$(dirname "${UU_CONFIG_DIR}")"
                touch "${UU_CONFIG_DIR}"
                chmod 0644 "${UU_CONFIG_DIR}"
                echo 'APT::Periodic::Update-Package-Lists "1";' >> "${UU_CONFIG_DIR}"
                echo 'APT::Periodic::Unattended-Upgrade "1";' >> "${UU_CONFIG_DIR}"
                systemctl unmask apt-daily.service apt-daily-upgrade.service
                systemctl enable apt-daily.service apt-daily-upgrade.service
                systemctl enable apt-daily.timer apt-daily-upgrade.timer
                systemctl restart --no-block apt-daily.timer apt-daily-upgrade.timer
                # this is the DOWNLOAD service
                # meaning we are wasting IO without even triggering an upgrade
                # -________________-
                systemctl restart --no-block apt-daily.service

            fi
            aptmarkWALinuxAgent unhold &
        elif isMarinerOrAzureLinux "$OS"; then
            if [ "${ENABLE_UNATTENDED_UPGRADES}" = "true" ]; then
                if [ "${IS_KATA}" = "true" ]; then
                    # Currently kata packages must be updated as a unit (including the kernel which requires a reboot). This can
                    # only be done reliably via image updates as of now so never enable automatic updates.
                    echo 'EnableUnattendedUpgrade is not supported by kata images, will not be enabled'
                elif isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
                    echo 'EnableUnattendedUpgrade is not supported by Azure Linux OS Guard, will not be enabled'
                else
                    # By default the dnf-automatic is service is notify only in Mariner.
                    # Enable the automatic install timer and the check-restart timer.
                    # Stop the notify only dnf timer since we've enabled the auto install one.
                    # systemctlDisableAndStop adds .service to the end which doesn't work on timers.
                    systemctl disable dnf-automatic-notifyonly.timer
                    systemctl stop dnf-automatic-notifyonly.timer
                    # At 6:00:00 UTC (1 hour random fuzz) download and install package updates.
                    systemctl unmask dnf-automatic-install.service || exit $ERR_SYSTEMCTL_START_FAIL
                    systemctl unmask dnf-automatic-install.timer || exit $ERR_SYSTEMCTL_START_FAIL
                    systemctlEnableAndStart dnf-automatic-install.timer 30 || exit $ERR_SYSTEMCTL_START_FAIL
                    # The check-restart service which will inform kured of required restarts should already be running
                fi
            fi
        fi
    fi
}

# The provisioning is split into two stages to support VHD image creation workflows:
#
# basePrep: Base image preparation
#   - Installs and configures all required components (kubelet, containerd, etc.)
#   - Sets up system configurations that are common across all nodes
#   - DOES NOT join the node to any cluster
#   - After this stage, users can add customizations (e.g., pre-pull additional container images)
#   - The VM can then be captured as a VHD image for use as a node pool base image
#
# nodePrep: Cluster integration and hardware setup
#   - Performs cluster-specific configurations
#   - Configures hardware-specific components (GPU drivers, MIG partitions, etc.)
#   - Establishes connection to the API server
#   - Joins the node to the cluster
#   - Only runs when actually provisioning a node, not when creating VHD images
#
# In typical deployments, both stages run sequentially during node provisioning.
# For VHD image creation workflows, only basePrep runs initially, and nodePrep runs later
# when nodes are created from that VHD image.
if [ ! -f /opt/azure/containers/base_prep.complete ]; then
    basePrep
else
    echo "Skipping basePrep - base_prep.complete file exists"
fi
if [ "${PRE_PROVISION_ONLY}" != "true" ]; then
    nodePrep
else
    echo "Skipping nodePrep - pre-provision only mode"
fi

echo "Custom script finished."
echo $(date),$(hostname), endcustomscript>>/opt/m
