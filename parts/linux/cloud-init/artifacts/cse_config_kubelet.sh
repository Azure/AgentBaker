#!/bin/bash

getPrimaryNicIP() {
    local ip=""
    export -f get_primary_nic_ip
    ip=$(get_primary_nic_ip)
    echo "${ip}"
}

generateSelfSignedKubeletServingCertificate() {
    mkdir -p "/etc/kubernetes/certs"

    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    openssl genrsa -out $KUBELET_SERVER_PRIVATE_KEY_PATH 2048
    openssl req -new -x509 -days 7300 -key $KUBELET_SERVER_PRIVATE_KEY_PATH -out $KUBELET_SERVER_CERT_PATH -subj "/CN=${NODE_NAME}" -addext "subjectAltName=DNS:${NODE_NAME}"
}

configureKubeletServing() {
    if [ "${ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION}" != "true" ]; then
        echo "kubelet serving certificate rotation is disabled, generating self-signed serving certificate with openssl"
        generateSelfSignedKubeletServingCertificate
        return 0
    fi

    KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL="kubernetes.azure.com/kubelet-serving-ca=cluster"
    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    # check if kubelet serving certificate rotation is disabled by customer-specified nodepool tags
    export -f should_disable_kubelet_serving_certificate_rotation
    DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION=$(should_disable_kubelet_serving_certificate_rotation)
    if [ "$?" -ne 0 ]; then
        echo "failed to determine if kubelet serving certificate rotation should be disabled by nodepool tags"
        exit $ERR_LOOKUP_DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION_TAG
    fi

    if [ "${DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION}" = "true" ]; then
        echo "kubelet serving certificate rotation is disabled by nodepool tags"

        # set --rotate-server-certificates flag and serverTLSBootstrap config file field to false
        echo "reconfiguring kubelet flags and config as needed"
        KUBELET_FLAGS="${KUBELET_FLAGS/--rotate-server-certificates=true/--rotate-server-certificates=false}"
        if [ "${KUBELET_CONFIG_FILE_ENABLED}" = "true" ]; then
            set +x
            KUBELET_CONFIG_FILE_CONTENT=$(echo "$KUBELET_CONFIG_FILE_CONTENT" | base64 -d | jq 'if .serverTLSBootstrap == true then .serverTLSBootstrap = false else . end' | base64)
            set -x
        fi

        # manually generate kubelet's self-signed serving certificate
        echo "generating self-signed serving certificate with openssl"
        generateSelfSignedKubeletServingCertificate

        # make sure to eliminate the kubelet serving node label
        echo "removing node label $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL"
        removeKubeletNodeLabel $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL
    else
        echo "kubelet serving certificate rotation is enabled"

        # remove the --tls-cert-file and --tls-private-key-file flags, which are incompatible with serving certificate rotation
        # NOTE: this step will not be needed once these flags are no longer defaulted by the bootstrapper
        echo "removing --tls-cert-file and --tls-private-key-file from kubelet flags"
        removeKubeletFlag "--tls-cert-file=$KUBELET_SERVER_CERT_PATH"
        removeKubeletFlag "--tls-private-key-file=$KUBELET_SERVER_PRIVATE_KEY_PATH"
        if [ "${KUBELET_CONFIG_FILE_ENABLED}" = "true" ]; then
            set +x
            KUBELET_CONFIG_FILE_CONTENT=$(echo "$KUBELET_CONFIG_FILE_CONTENT" | base64 -d | jq 'del(.tlsCertFile)' | jq 'del(.tlsPrivateKeyFile)' | base64)
            set -x
        fi

        # make sure to add the kubelet serving node label
        echo "adding node label $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL if needed"
        addKubeletNodeLabel $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL
    fi
}

ensureKubeCACert() {
    KUBE_CA_FILE="/etc/kubernetes/certs/ca.crt"
    mkdir -p "$(dirname "${KUBE_CA_FILE}")"
    echo "${KUBE_CA_CRT}" | base64 -d > "${KUBE_CA_FILE}"
    chmod 0600 "${KUBE_CA_FILE}"
}

# file paths defined outside so configureAndEnableSecureTLSBootstrapping can be unit tested
SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE="/etc/default/secure-tls-bootstrap"
SECURE_TLS_BOOTSTRAPPING_DROP_IN="/etc/systemd/system/secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
configureAndEnableSecureTLSBootstrapping() {
    BOOTSTRAP_CLIENT_FLAGS="--aad-resource=${SECURE_TLS_BOOTSTRAPPING_AAD_RESOURCE:-$AKS_AAD_SERVER_APP_ID} --apiserver-fqdn=${API_SERVER_NAME} --cloud-provider-config=${AZURE_JSON_PATH}"
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --user-assigned-identity-id=$SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_VALIDATE_KUBECONFIG_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --validate-kubeconfig-timeout=${SECURE_TLS_BOOTSTRAPPING_VALIDATE_KUBECONFIG_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_ACCESS_TOKEN_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-access-token-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_ACCESS_TOKEN_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_INSTANCE_DATA_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-instance-data-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_INSTANCE_DATA_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_NONCE_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-nonce-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_NONCE_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_ATTESTED_DATA_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-attested-data-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_ATTESTED_DATA_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_CREDENTIAL_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-credential-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_CREDENTIAL_TIMEOUT}"
    fi

    mkdir -p "$(dirname "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}")"
    touch "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    chmod 0600 "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    echo "BOOTSTRAP_FLAGS=${BOOTSTRAP_CLIENT_FLAGS}" > "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    if [ -n "${AZURE_ENVIRONMENT_FILEPATH}" ]; then
        echo "AZURE_ENVIRONMENT_FILEPATH=${AZURE_ENVIRONMENT_FILEPATH}" >> "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    fi

    mkdir -p "$(dirname "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}")"
    touch "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}"
    chmod 0600 "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}"
    cat > "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}" <<EOF
[Unit]
Before=kubelet.service
[Service]
EnvironmentFile=${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}
[Install]
# this configuration has secure-tls-bootstrap.service only start when kubelet.service is started
# once bootstrap tokens are no longer a fallback, kubelet.service needs to be a RequiredBy=
WantedBy=kubelet.service
EOF

    # enable the service so it runs ahead of kubelet on next boot; do not start it now
    if ! retrycmd_if_failure 120 5 25 systemctl enable secure-tls-bootstrap; then
        echo "secure-tls-bootstrap could not be enabled by systemctl"
        exit $ERR_SECURE_TLS_BOOTSTRAP_ENABLE_FAILURE
    fi

    # once bootstrap tokens are no longer a fallback, we can unset TLS_BOOTSTRAP_TOKEN here if needed
}

configureKubeletAndKubectl() {
    # Install kubelet and kubectl binaries from URL:
    # 1. For Custom Kube binary or Private Kube binary.
    # 2. If k8s version < 1.34.0, skip_bypass_k8s_version_check != true, and not Flatcar (which falls back to URL later).
    # 3. For Azure Linux v2 due to lack of PMC packages (if not network isolated).
    if [ -n "${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}" ] || [ -n "${PRIVATE_KUBE_BINARY_DOWNLOAD_URL}" ] ||
       { ! isFlatcar && ! isACL && [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != true ] && ! semverCompare "${KUBERNETES_VERSION:-0.0.0}" 1.34.0; } ||
       { isMarinerOrAzureLinux && [ "${OS_VERSION}" = 2.0 ] && [ -z "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; }
    then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromURL" installKubeletKubectlFromURL
    elif [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromBootstrapProfileRegistry" "installKubeletKubectlFromBootstrapProfileRegistry ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ${KUBERNETES_VERSION}"
    elif [ "$(type -t installKubeletKubectlFromPkg)" = function ]; then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromPkg" "installKubeletKubectlFromPkg ${KUBERNETES_VERSION}"
    else
        echo "installKubeletKubectlFromPkg is not defined for this OS"
        exit $ERR_K8S_INSTALL_ERR
    fi
}

ensurePodInfraContainerImage() {
    POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR="/opt/pod-infra-container-image/downloads"
    POD_INFRA_CONTAINER_IMAGE_TAR="/opt/pod-infra-container-image/pod-infra-container-image.tar"

    waitForContainerdReady || exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE

    pod_infra_container_image=$(get_sandbox_image)

    if [ -z "${pod_infra_container_image}" ]; then
        echo "Failed to recognize pod infra container image"
        exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE
    fi

    echo "Checking if $pod_infra_container_image already exists locally..."
    if ctr -n k8s.io images list -q | grep -q "^${pod_infra_container_image}$"; then
        echo "Image $pod_infra_container_image already exists locally, skipping pull"
        return 0
    fi
    base_name="${pod_infra_container_image%@:*}"
    base_name="${pod_infra_container_image%:*}"
    tag="local"

    MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE:-mcr.microsoft.com}"
    MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE%/}"
    image="${pod_infra_container_image//${MCR_REPOSITORY_BASE}/${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}}"
    acr_url=$(echo "$image" | cut -d/ -f1)

    mkdir -p ${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR}

    echo "Pulling with authentication for $image"
    retrycmd_cp_oci_layout_with_oras 10 5 "${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR}" "$tag" "$image" || exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE

    tar -cvf ${POD_INFRA_CONTAINER_IMAGE_TAR} -C ${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR} .
    if ctr -n k8s.io image import --base-name $base_name ${POD_INFRA_CONTAINER_IMAGE_TAR}; then
        ctr -n k8s.io image tag "${base_name}:${tag}" "${pod_infra_container_image}"
        echo "Successfully imported $pod_infra_container_image"
        labelContainerImage "${pod_infra_container_image}" "io.cri-containerd.pinned" "pinned"
    else
        echo "Failed to import $pod_infra_container_image"
        exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE
    fi

    rm -rf ${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR}
    rm -f ${POD_INFRA_CONTAINER_IMAGE_TAR}
}

validateKubeletNodeLabels() {
    local labels="$1"
    local validated_labels=""
    local delimiter=""

    # Return empty if no labels provided
    if [ -z "$labels" ]; then
        echo "No labels found in KUBELET_NODE_LABELS"
        return 0
    fi

    # Split labels by comma and process each
    IFS=',' read -ra LABEL_ARRAY <<< "$labels"
    for label in "${LABEL_ARRAY[@]}"; do
        # Split each label into key and value
        # shellcheck disable=SC3010
        if [[ "$label" == *"="* ]]; then
            key="${label%%=*}"
            value="${label#*=}"

            # Check if key length exceeds 63 characters
            if [ ${#key} -gt 63 ]; then
                echo "Warning: Label key '$key' exceeds 63 characters, truncating to 63 characters" >&2
                key="${key:0:63}"
            fi

            # Rebuild the label with potentially truncated key
            validated_labels="${validated_labels}${delimiter}${key}=${value}"
        fi

        # Set delimiter for subsequent labels
        delimiter=","
    done

    # Update the global variable with validated labels
    KUBELET_NODE_LABELS="$validated_labels"
}

ensureKubelet() {
    KUBELET_DEFAULT_FILE=/etc/default/kubelet
    mkdir -p /etc/default

    # In k8s >= 1.29 kubelet no longer sets node internalIP when using external cloud provider
    # https://github.com/kubernetes/kubernetes/pull/121028
    # This regresses node startup performance in Azure CNI Overlay and Podsubnet clusters, which require the node to be
    # assigned an internal IP before configuring pod networking.
    # To improve node startup performance, explicitly set `--node-ip` to the IP returned from IMDS so kubelet sets
    # the internal IP when it registers the node.
    # If this fails, skip setting --node-ip, which is safe because cloud-node-manager will assign it later anyway.
    if semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.29.0"; then
        logs_to_events "AKS.CSE.ensureKubelet.setKubeletNodeIPFlag" setKubeletNodeIPFlag
    fi

    # systemd watchdog support was added in 1.32.0: https://github.com/kubernetes/kubernetes/pull/127566
    # This is needed to ensure kubelet is restarted if it becomes unresponsive
    if semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.32.0"; then
        tee "/etc/systemd/system/kubelet.service.d/10-watchdog.conf" > /dev/null <<'EOF'
[Service]
WatchdogSec=60s
EOF
    fi

    echo "KUBELET_FLAGS=${KUBELET_FLAGS}" > "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_REGISTER_SCHEDULABLE=true" >> "${KUBELET_DEFAULT_FILE}"
    echo "NETWORK_POLICY=${NETWORK_POLICY}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_IMAGE=${KUBELET_IMAGE}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_NODE_LABELS=${KUBELET_NODE_LABELS}" >> "${KUBELET_DEFAULT_FILE}"
    if [ -n "${AZURE_ENVIRONMENT_FILEPATH}" ]; then
        echo "AZURE_ENVIRONMENT_FILEPATH=${AZURE_ENVIRONMENT_FILEPATH}" >> "${KUBELET_DEFAULT_FILE}"
    fi
    chmod 0600 "${KUBELET_DEFAULT_FILE}"

    BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig

    # to ensure we don't expose bootstrap token secrets in provisioning logs
    set +x

    if [ -n "${TLS_BOOTSTRAP_TOKEN:-}" ]; then
        echo "using bootstrap token to generate a bootstrap-kubeconfig"

        CREDENTIAL_VALIDATION_DROP_IN="/etc/systemd/system/kubelet.service.d/10-credential-validation.conf"
        mkdir -p "$(dirname "${CREDENTIAL_VALIDATION_DROP_IN}")"
        touch "${CREDENTIAL_VALIDATION_DROP_IN}"
        chmod 0600 "${CREDENTIAL_VALIDATION_DROP_IN}"
        tee "${CREDENTIAL_VALIDATION_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="CREDENTIAL_VALIDATION_KUBE_CA_FILE=/etc/kubernetes/certs/ca.crt"
Environment="CREDENTIAL_VALIDATION_APISERVER_URL=https://${API_SERVER_NAME}:443"
EOF

        KUBELET_TLS_DROP_IN="/etc/systemd/system/kubelet.service.d/10-tlsbootstrap.conf"
        mkdir -p "$(dirname "${KUBELET_TLS_DROP_IN}")"
        touch "${KUBELET_TLS_DROP_IN}"
        chmod 0600 "${KUBELET_TLS_DROP_IN}"
        tee "${KUBELET_TLS_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
EOF
        mkdir -p "$(dirname "${BOOTSTRAP_KUBECONFIG_FILE}")"
        touch "${BOOTSTRAP_KUBECONFIG_FILE}"
        chmod 0644 "${BOOTSTRAP_KUBECONFIG_FILE}"
        tee "${BOOTSTRAP_KUBECONFIG_FILE}" > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://${API_SERVER_NAME}:443
users:
- name: kubelet-bootstrap
  user:
    token: "${TLS_BOOTSTRAP_TOKEN:-}"
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF
    else
        echo "generating kubeconfig referencing the provided kubelet client certificate"

        KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
        mkdir -p "$(dirname "${KUBECONFIG_FILE}")"
        touch "${KUBECONFIG_FILE}"
        chmod 0644 "${KUBECONFIG_FILE}"
        tee "${KUBECONFIG_FILE}" > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://${API_SERVER_NAME}:443
users:
- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key
contexts:
- context:
    cluster: localcluster
    user: client
  name: localclustercontext
current-context: localclustercontext
EOF
    fi

    set -x

    KUBELET_RUNTIME_CONFIG_SCRIPT_FILE=/opt/azure/containers/kubelet.sh
    tee "${KUBELET_RUNTIME_CONFIG_SCRIPT_FILE}" > /dev/null <<'EOF'
#!/bin/bash
# Disallow container from reaching out to the special IP address 168.63.129.16
# for TCP protocol (which http uses)
#
# 168.63.129.16 contains protected settings that have priviledged info.
# HostGAPlugin (Host-GuestAgent-Plugin) is a web server process that runs on the physical host that serves the operational and diagnostic needs of the in-VM Guest Agent.
# IT listens on both port 80 and 32526 hence access is only needed for agent but not the containers.
#
# The host can still reach 168.63.129.16 because it goes through the OUTPUT chain, not FORWARD.
#
# Note: we should not block all traffic to 168.63.129.16. For example UDP traffic is still needed
# for DNS.

for port in 80 32526; do
    iptables -C FORWARD -d 168.63.129.16 -p tcp --dport "$port" -j DROP 2>/dev/null ||
        iptables -I FORWARD -d 168.63.129.16 -p tcp --dport "$port" -j DROP
done
EOF

    # As iptables rule will be cleaned every time the node is restarted, we need to ensure the rule is applied every time kubelet is started.
    primaryNicIP=$(logs_to_events "AKS.CSE.ensureKubelet.getPrimaryNicIP" getPrimaryNicIP)
    ENSURE_IMDS_RESTRICTION_DROP_IN="/etc/systemd/system/kubelet.service.d/10-ensure-imds-restriction.conf"
    mkdir -p "$(dirname "${ENSURE_IMDS_RESTRICTION_DROP_IN}")"
    touch "${ENSURE_IMDS_RESTRICTION_DROP_IN}"
    chmod 0600 "${ENSURE_IMDS_RESTRICTION_DROP_IN}"
    tee "${ENSURE_IMDS_RESTRICTION_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="PRIMARY_NIC_IP=${primaryNicIP}"
Environment="ENABLE_IMDS_RESTRICTION=${ENABLE_IMDS_RESTRICTION}"
Environment="INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE=${INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE}"
EOF

    # check if kubelet flags contain image-credential-provider-config and image-credential-provider-bin-dir
    # shellcheck disable=SC3010
    if [[ $KUBELET_FLAGS == *"image-credential-provider-config"* && $KUBELET_FLAGS == *"image-credential-provider-bin-dir"* ]]; then
        echo "Configure credential provider for both image-credential-provider-config and image-credential-provider-bin-dir flags are specified in KUBELET_FLAGS"
        logs_to_events "AKS.CSE.ensureKubelet.configCredentialProvider" configCredentialProvider
        # Install credential provider from URL:
        # 1. If k8s version < 1.33.0, skip_bypass_k8s_version_check != true, and not Flatcar (which falls back to URL later).
        # 2. For Azure Linux v2 due to lack of PMC packages (if not network isolated).
        if { ! isFlatcar && ! isACL && [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != true ] && ! semverCompare "${KUBERNETES_VERSION:-0.0.0}" 1.33.0; } ||
           { isMarinerOrAzureLinux && [ "${OS_VERSION}" = 2.0 ] && [ -z "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; }
        then
            logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromUrl" installCredentialProviderFromUrl
        elif [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
            # For network isolated clusters, try distro packages first and fallback to binary installation
            logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromBootstrapProfileRegistry" installCredentialProviderPackageFromBootstrapProfileRegistry ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ${KUBERNETES_VERSION}
        elif [ "$(type -t installCredentialProviderFromPkg)" = function ]; then
            logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromPkg" "installCredentialProviderFromPkg ${KUBERNETES_VERSION}"
        else
            echo "installCredentialProviderFromPkg is not defined for this OS"
            exit $ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT
        fi
    fi

    # kubelet cannot pull pause image from anonymous disabled registry during runtime
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        logs_to_events "AKS.CSE.ensureKubelet.ensurePodInfraContainerImage" ensurePodInfraContainerImage
    fi

    local tls_bootstrapping_start_time_filepath="/opt/azure/containers/tls-bootstrap-start-time"
    date +"%F %T.%3N" > "${tls_bootstrapping_start_time_filepath}"

    # Node Memory Hardening (F2/F5): idempotent refresh for PIS real nodes booting
    # from older cached VHDs where basePrep (ensureContainerd) may not have created
    # the Slice= drop-ins. No-op when neither value is set (non-hardened pools).
    resolveKubeletReservedCgroups
    if [ -n "${KUBE_RESERVED_CGROUP}" ] || [ -n "${SYSTEM_RESERVED_CGROUP}" ]; then
        if ! logs_to_events "AKS.CSE.ensureKubelet.ensureKubeletCgroupHierarchy" ensureKubeletCgroupHierarchy; then
            exit $ERR_KUBELET_START_FAIL
        fi
    fi

    # Refresh --runtime-cgroups for PIS nodes where basePrep may have baked an older
    # 10-containerd-base-flag.conf pointing at /system.slice/containerd.service.
    local containerd_runtime_cgroups="/system.slice/containerd.service"
    if [ "${KUBE_RESERVED_CGROUP:-}" = "/kubereserved.slice" ] || [ "${KUBE_RESERVED_CGROUP:-}" = "kubereserved.slice" ]; then
        containerd_runtime_cgroups="/kubereserved.slice/containerd.service"
    fi
    tee "/etc/systemd/system/kubelet.service.d/10-containerd-base-flag.conf" > /dev/null <<EOF
[Service]
Environment="KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=${containerd_runtime_cgroups}"
EOF

    if ! systemctl daemon-reload; then
        exit $ERR_KUBELET_START_FAIL
    fi
    # start kubelet.service without waiting for the main process to start, though check whether it has entered a failed state after enablement
    if ! systemctlEnableAndStartNoBlock kubelet 240; then
        # append kubelet status to CSE output to ensure we can see it
        rm -f "${tls_bootstrapping_start_time_filepath}"
        journalctl -u kubelet.service --no-pager || true
        exit $ERR_KUBELET_START_FAIL
    fi

    # start measure-tls-bootstrapping-latency.service without waiting for the main process to start, while ignoring any failures
    if ! systemctlEnableAndStartNoBlock measure-tls-bootstrapping-latency 30; then
        rm -f "${tls_bootstrapping_start_time_filepath}"
        echo "failed to start measure-tls-bootstrapping-latency.service"
    fi
}

# Internal function that writes credential provider config to a specified path
# This function is extracted to allow unit testing without root permissions
# Usage: writeCredentialProviderConfig <config_file_path>
writeCredentialProviderConfig() {
    if [ -z "$1" ]; then
        echo "Error: writeCredentialProviderConfig requires config file path as argument"
        return 1
    fi
    local config_file_path="$1"
    mkdir -p "$(dirname "${config_file_path}")"
    touch "${config_file_path}"

    # Prepare identity binding configuration if enabled (including leading newlines)
    local ib_token_attributes=""
    local ib_args=""
    local ib_args_list=()
    if [ "${SERVICE_ACCOUNT_IMAGE_PULL_ENABLED}" = "true" ]; then
        ib_token_attributes="
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id"
        # Build identity binding args list using an array to avoid word splitting
        ib_args_list=( "--ib-sni-name=${IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI}" )
        [ -n "${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID}" ] && ib_args_list+=( "--ib-default-client-id=${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID}" )
        [ -n "${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID}" ] && ib_args_list+=( "--ib-default-tenant-id=${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID}" )
        ib_args_list+=( "--ib-apiserver-ip=${API_SERVER_NAME}" )
        # Format args as YAML list items with proper indentation
        for arg in "${ib_args_list[@]}"; do
            ib_args="${ib_args}
      - ${arg}"
        done
    fi
    # matchImages and argument for network isolated cluster
    local bootstrap_container_registry_match_image=""
    local bootstrap_container_registry_args=""
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
      MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE:=mcr.microsoft.com}"
      MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE%/}"
      bootstrap_container_registry_match_image="
      - \"${MCR_REPOSITORY_BASE}\""
      bootstrap_container_registry_args="
      - --registry-mirror=${MCR_REPOSITORY_BASE}:${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}"
    fi

    if [ -n "$AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX" ]; then
        echo "configure credential provider for custom cloud"
        tee "${config_file_path}" > /dev/null <<EOF
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"
      - "*$AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"${bootstrap_container_registry_match_image}
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1${ib_token_attributes}
    args:
      - /etc/kubernetes/azure.json${bootstrap_container_registry_args}${ib_args}
EOF
    else
        echo "configure credential provider with default settings"
        tee "${config_file_path}" > /dev/null <<EOF
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*.*.geo.azurecr.io"
      - "*.*.geo.azurecr.cn"
      - "*.*.geo.azurecr.de"
      - "*.*.geo.azurecr.us"${bootstrap_container_registry_match_image}
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1${ib_token_attributes}
    args:
      - /etc/kubernetes/azure.json${bootstrap_container_registry_args}${ib_args}
EOF
    fi
}

configCredentialProvider() {
    writeCredentialProviderConfig "/var/lib/kubelet/credential-provider-config.yaml"
}

setKubeletNodeIPFlag() {
    local imdsOutput
    export -f get_imds_network_metadata
    imdsOutput=$(get_imds_network_metadata)
    nodeIPAddrs=()
    ipv4Addr=$(echo $imdsOutput | jq -r '.[0].ipv4.ipAddress[0].privateIpAddress // ""')
    [ -n "$ipv4Addr" ] && nodeIPAddrs+=("$ipv4Addr")
    ipv6Addr=$(echo $imdsOutput | jq -r '.[0].ipv6.ipAddress[0].privateIpAddress // ""')
    [ -n "$ipv6Addr" ] && nodeIPAddrs+=("$ipv6Addr")
    nodeIPArg=$(IFS=, ; echo "${nodeIPAddrs[*]}") # join, comma-separated
    if [ -n "$nodeIPArg" ]; then
        echo "Adding --node-ip=$nodeIPArg to kubelet flags"
        KUBELET_FLAGS="$KUBELET_FLAGS --node-ip=$nodeIPArg"
    fi
}

#EOF
