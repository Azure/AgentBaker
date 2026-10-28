#!/bin/bash

reconcileKubeletConfigFlags() (
    set +x
    local runtime_cgroups="$1"
    local root_dir="${2:-}"
    local managed_drop_in="${root_dir}/etc/systemd/system/kubelet.service.d/11-kubelet-config-flags.conf"
    local owned=false
    if [ -e "${managed_drop_in}" ] || [ -L "${managed_drop_in}" ]; then
        [ -f "${managed_drop_in}" ] && [ ! -L "${managed_drop_in}" ] || return 0
        local ownership_marker content_digest
        IFS= read -r ownership_marker < "${managed_drop_in}" || return 0
        content_digest=$(tail -n +2 "${managed_drop_in}" | sha256sum) || return 1
        [ "${ownership_marker}" = "#AKS kubelet flag omissions v1 sha256:${content_digest%% *}" ] || return 0
        owned=true
    fi
    local content
    content=$(renderKubeletConfigFlags "${runtime_cgroups}" "${root_dir}") || return 1
    if [ -z "${content}" ]; then
        if [ "${owned}" = true ]; then
            rm -f "${managed_drop_in}" || return 1
        fi
        return 0
    fi
    local temporary_drop_in
    temporary_drop_in=$(mktemp "${managed_drop_in}.XXXXXX") || return 1
    trap 'rm -f "${temporary_drop_in}"' EXIT
    local content_digest
    content_digest=$(printf '%s\n' "${content}" | sha256sum) || return 1
    printf '#AKS kubelet flag omissions v1 sha256:%s\n%s\n' "${content_digest%% *}" "${content}" > "${temporary_drop_in}" || return 1
    chmod 0600 "${temporary_drop_in}" || return 1
    mv -f "${temporary_drop_in}" "${managed_drop_in}"
)

renderKubeletConfigFlags() (
    set +x
    local runtime_cgroups="$1"
    local root_dir="${2:-}"
    local service_file="${root_dir}/etc/systemd/system/kubelet.service"
    local drop_in_dir="${service_file}.d"
    local config_file="${root_dir}/etc/default/kubeletconfig.json"
    local default_file="${root_dir}/etc/default/kubelet"
    local encoded_flags="${KUBELET_FLAGS_TO_OMIT:-}"
    if [ "${KUBELET_CONFIG_FILE_ENABLED:-}" != "true" ] || [ -z "${encoded_flags}" ] || [ "${#encoded_flags}" -gt 1024 ]; then
        return 0
    fi
    case "${KUBELET_FLAGS:-}" in
        *--config*) return 0 ;;
    esac
    case $((${#encoded_flags} % 4)) in
        1) return 0 ;;
        2) encoded_flags="${encoded_flags}==" ;;
        3) encoded_flags="${encoded_flags}=" ;;
    esac
    local requested_flags
    requested_flags=$(printf '%s' "${encoded_flags}" | base64 -d 2>/dev/null) || return 0
    printf '%s' "${requested_flags}" | jq -e 'type == "array" and length <= 16 and all(.[]; type == "string" and test("^--[a-z][a-z0-9-]{0,62}$"))' >/dev/null 2>&1 || return 0
    jq -e 'type == "object"' "${config_file}" >/dev/null 2>&1 || return 0
    local expected_config_drop_in='[Service]
Environment="KUBELET_CONFIG_FILE_FLAGS=--config /etc/default/kubeletconfig.json"'
    [ "$(cat "${drop_in_dir}/10-componentconfig.conf" 2>/dev/null)" = "${expected_config_drop_in}" ] || return 0
    [ -f "${service_file}" ] && [ ! -L "${service_file}" ] || return 0
    local service_digest
    service_digest=$(sha256sum "${service_file}" 2>/dev/null) || return 0
    case "${service_digest%% *}" in
        98172d7111cb1d7d9b9b8469ca1a7bd2aebbb0a48bc8f6633c5210f4df7dd1e9|e05dba2eee92f4c56c5dd3ff072b745b06c80a3399d0401ae50440a723db9e97) ;;
        *) return 0 ;;
    esac
    [ -f "${default_file}" ] && [ ! -L "${default_file}" ] || return 0
    [ "$(grep -c '^KUBELET_FLAGS=' "${default_file}")" = 1 ] || return 0
    if grep -qE -- "--config|[\"'\\\\]" "${default_file}" ||
       grep -qEv '^(KUBELET_FLAGS|KUBELET_REGISTER_SCHEDULABLE|NETWORK_POLICY|KUBELET_IMAGE|KUBELET_NODE_LABELS|AZURE_ENVIRONMENT_FILEPATH)=[[:print:]]*$' "${default_file}"; then
        return 0
    fi
    local expected_runtime_drop_in="[Service]
Environment=\"KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=${runtime_cgroups}\""
    [ "$(cat "${drop_in_dir}/10-containerd-base-flag.conf" 2>/dev/null)" = "${expected_runtime_drop_in}" ] || return 0
    local expected_cgroup_drop_in='[Service]
Environment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd"'
    local expected_tls_drop_in='[Service]
Environment="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"'
    local expected_watchdog_drop_in='[Service]
WatchdogSec=60s'
    local expected_bindmount_drop_in='[Unit]
Requires=bind-mount.service
After=bind-mount.service'
    local expected_reserved_slice_drop_in='[Unit]
Wants=kubereserved.slice
After=kubereserved.slice

[Service]
Slice=kubereserved.slice'
    local expected_credential_drop_in='[Service]
Environment="CREDENTIAL_VALIDATION_KUBE_CA_FILE=/etc/kubernetes/certs/ca.crt"
Environment="CREDENTIAL_VALIDATION_APISERVER_URL=https://APISERVER:443"'
    local expected_imds_drop_in='[Service]
Environment="PRIMARY_NIC_IP=VALUE"
Environment="ENABLE_IMDS_RESTRICTION=VALUE"
Environment="INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE=VALUE"'
    local unit_directory unit_file drop_in normalized_drop_in
    for unit_directory in /etc/systemd/system.control /run/systemd/system.control /run/systemd/transient /run/systemd/generator.early; do
        [ ! -e "${root_dir}${unit_directory}/kubelet.service" ] && [ ! -L "${root_dir}${unit_directory}/kubelet.service" ] || return 0
    done
    for unit_directory in /etc/systemd/system.control /run/systemd/system.control /run/systemd/transient /run/systemd/generator.early \
        /etc/systemd/system /etc/systemd/system.attached /run/systemd/system /run/systemd/system.attached /run/systemd/generator \
        /usr/local/lib/systemd/system /usr/lib/systemd/system /lib/systemd/system /run/systemd/generator.late; do
        for unit_file in "${root_dir}${unit_directory}/"*.service; do
            if [ -L "${unit_file}" ] && [ "${unit_file}" -ef "${service_file}" ]; then
                return 0
            fi
        done
        for drop_in in "${root_dir}${unit_directory}/service.d/"*.conf "${root_dir}${unit_directory}/kubelet.service.d/"*.conf; do
            [ -e "${drop_in}" ] || [ -L "${drop_in}" ] || continue
            [ -f "${drop_in}" ] && [ ! -L "${drop_in}" ] || return 0
            case "${drop_in}" in
                "${drop_in_dir}/11-kubelet-config-flags.conf"|"${drop_in_dir}/10-componentconfig.conf"|"${drop_in_dir}/10-containerd-base-flag.conf") continue ;;
                "${drop_in_dir}/10-cgroupv2.conf")
                    [ "$(cat "${drop_in}")" = "${expected_cgroup_drop_in}" ] || return 0
                    continue ;;
                "${drop_in_dir}/10-tlsbootstrap.conf")
                    [ "$(cat "${drop_in}")" = "${expected_tls_drop_in}" ] || return 0
                    continue ;;
                "${drop_in_dir}/10-watchdog.conf")
                    [ "$(cat "${drop_in}")" = "${expected_watchdog_drop_in}" ] || return 0
                    continue ;;
                "${drop_in_dir}/10-bindmount.conf")
                    [ "$(cat "${drop_in}")" = "${expected_bindmount_drop_in}" ] || return 0
                    continue ;;
                "${drop_in_dir}/10-kubereserved-slice.conf")
                    [ "$(cat "${drop_in}")" = "${expected_reserved_slice_drop_in}" ] || return 0
                    continue ;;
                "${drop_in_dir}/10-credential-validation.conf")
                    normalized_drop_in=$(sed -E 's|^(Environment="CREDENTIAL_VALIDATION_APISERVER_URL=https://)[a-zA-Z0-9.-]+(:443")$|\1APISERVER\2|' "${drop_in}")
                    [ "${normalized_drop_in}" = "${expected_credential_drop_in}" ] || return 0
                    continue ;;
                "${drop_in_dir}/10-ensure-imds-restriction.conf")
                    normalized_drop_in=$(sed -E 's/^(Environment="PRIMARY_NIC_IP=)[0-9.]+"$/\1VALUE"/; s/^(Environment="(ENABLE_IMDS_RESTRICTION|INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE)=)(true|false)?"$/\1VALUE"/' "${drop_in}")
                    [ "${normalized_drop_in}" = "${expected_imds_drop_in}" ] || return 0
                    continue ;;
                *) return 0 ;;
            esac
        done
    done

    local exec_start
    exec_start=$(awk '/^ExecStart=/ { inside = 1 } inside { print } inside && !/\\$/ { inside = 0 }' "${service_file}" 2>/dev/null)

    local omit_timeout=false
    local omit_endpoint=false
    local omit_cgroup=false
    local service_changed=false
    local flag
    for flag in $(printf '%s' "${requested_flags}" | jq -r '.[]'); do
        case "${flag}" in
            --enable-server)
                if [ -n "${exec_start}" ] && jq -e '.enableServer == true' "${config_file}" >/dev/null; then
                    exec_start=$(printf '%s\n' "${exec_start}" | grep -vxF "        --enable-server \\")
                    service_changed=true
                fi
                ;;
            --volume-plugin-dir)
                if [ -n "${exec_start}" ] && jq -e '.volumePluginDir == "/etc/kubernetes/volumeplugins"' "${config_file}" >/dev/null; then
                    exec_start=$(printf '%s\n' "${exec_start}" | grep -vxF "        --volume-plugin-dir=/etc/kubernetes/volumeplugins \\")
                    service_changed=true
                fi
                ;;
            --cgroup-driver)
                if [ "${NEEDS_CGROUPV2:-}" = "true" ] && [ "$(cat "${drop_in_dir}/10-cgroupv2.conf" 2>/dev/null)" = "${expected_cgroup_drop_in}" ] && jq -e '.cgroupDriver == "systemd"' "${config_file}" >/dev/null; then
                    omit_cgroup=true
                fi
                ;;
            --runtime-request-timeout)
                if jq -e '.runtimeRequestTimeout == "15m"' "${config_file}" >/dev/null; then
                    omit_timeout=true
                fi
                ;;
            --container-runtime-endpoint)
                if jq -e '.containerRuntimeEndpoint == "unix:///run/containerd/containerd.sock"' "${config_file}" >/dev/null; then
                    omit_endpoint=true
                fi
                ;;
        esac
    done

    if [ "${service_changed}" = false ] && [ "${omit_cgroup}" = false ] && [ "${omit_timeout}" = false ] && [ "${omit_endpoint}" = false ]; then
        return 0
    fi
    local content='[Service]'
    if [ "${service_changed}" = true ]; then
        content="${content}
ExecStart=
${exec_start}"
    fi
    if [ "${omit_cgroup}" = true ]; then
        content="${content}
Environment=\"KUBELET_CGROUP_FLAGS=\""
    fi
    if [ "${omit_timeout}" = true ] || [ "${omit_endpoint}" = true ]; then
        local runtime_flags=""
        if [ "${omit_timeout}" = false ]; then
            runtime_flags='--runtime-request-timeout=15m '
        fi
        if [ "${omit_endpoint}" = false ]; then
            runtime_flags="${runtime_flags}--container-runtime-endpoint=unix:///run/containerd/containerd.sock "
        fi
        content="${content}
Environment=\"KUBELET_CONTAINERD_FLAGS=${runtime_flags}--runtime-cgroups=${runtime_cgroups}\""
    fi
    printf '%s\n' "${content}"
)

if [ "${1:-}" = --apply ]; then
    reconcileKubeletConfigFlags "${2:-}" "${3:-}"
fi
