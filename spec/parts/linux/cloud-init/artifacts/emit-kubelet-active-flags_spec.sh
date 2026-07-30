#!/bin/bash

Describe 'emit-kubelet-active-flags.sh'
    # Source the script without running its entrypoint (emitKubeletActiveFlagsEvent), so the
    # individual functions can be unit-tested in isolation.
    EMIT_KUBELET_ACTIVE_FLAGS_SOURCE_ONLY=1
    Include "./parts/linux/cloud-init/artifacts/emit-kubelet-active-flags.sh"

    Describe 'getKubeletActiveFlagsJSON'
        It 'reads flags from /etc/default/kubelet and config file when both exist'
            tmpdir="$(mktemp -d)"
            echo 'KUBELET_FLAGS=--cloud-provider=external --max-pods=110 --cgroup-driver=systemd' > "${tmpdir}/kubelet"
            echo '{"maxPods":110,"clusterDNS":["10.0.0.10"]}' > "${tmpdir}/kubeletconfig.json"
            KUBELET_DEFAULT_FILE="${tmpdir}/kubelet"
            KUBELET_CONFIG_JSON_PATH="${tmpdir}/kubeletconfig.json"
            When call getKubeletActiveFlagsJSON
            The output should include '"uses_config_file":true'
            The output should include '"cloud-provider":"external"'
            The output should include '"max-pods":"110"'
            The output should include '"cgroup-driver":"systemd"'
            The output should include '"flag_count":3'
            The output should include '"maxPods":110'
            The output should include '"clusterDNS"'
        End

        It 'reports uses_config_file=false when config file does not exist'
            tmpdir="$(mktemp -d)"
            echo 'KUBELET_FLAGS=--cloud-provider=external --max-pods=110' > "${tmpdir}/kubelet"
            KUBELET_DEFAULT_FILE="${tmpdir}/kubelet"
            KUBELET_CONFIG_JSON_PATH="${tmpdir}/nonexistent.json"
            When call getKubeletActiveFlagsJSON
            The output should include '"uses_config_file":false'
            The output should include '"cloud-provider":"external"'
            The output should include '"flag_count":2'
            The output should include '"config_file":{}'
        End

        It 'returns empty flags when /etc/default/kubelet does not exist'
            tmpdir="$(mktemp -d)"
            KUBELET_DEFAULT_FILE="${tmpdir}/nonexistent"
            KUBELET_CONFIG_JSON_PATH="${tmpdir}/nonexistent.json"
            When call getKubeletActiveFlagsJSON
            The output should include '"flag_count":0'
            The output should include '"uses_config_file":false'
            The output should include '"flags":{}'
        End

        It 'truncates config_file when payload exceeds size cap'
            tmpdir="$(mktemp -d)"
            echo 'KUBELET_FLAGS=--max-pods=110' > "${tmpdir}/kubelet"
            # Create a large config file that will exceed 3000 bytes
            printf '{\n' > "${tmpdir}/kubeletconfig.json"
            for i in $(seq 1 100); do
                printf '"key%s": "%s",\n' "$i" "$(printf 'x%.0s' $(seq 1 50))" >> "${tmpdir}/kubeletconfig.json"
            done
            printf '"last": "value"\n}\n' >> "${tmpdir}/kubeletconfig.json"
            KUBELET_DEFAULT_FILE="${tmpdir}/kubelet"
            KUBELET_CONFIG_JSON_PATH="${tmpdir}/kubeletconfig.json"
            When call getKubeletActiveFlagsJSON
            The output should include '"config_file":"truncated:exceeded_size_cap"'
            The output should include '"max-pods":"110"'
            The stderr should include 'dropping config_file content'
        End
    End

    Describe 'emitKubeletActiveFlagsEvent'
        It 'writes a guest agent event whose Message is the kubelet config JSON'
            tmpdir="$(mktemp -d)"
            EVENTS_LOGGING_DIR="${tmpdir}/events/"
            echo 'KUBELET_FLAGS=--cloud-provider=external' > "${tmpdir}/kubelet"
            echo '{"maxPods":110}' > "${tmpdir}/kubeletconfig.json"
            KUBELET_DEFAULT_FILE="${tmpdir}/kubelet"
            KUBELET_CONFIG_JSON_PATH="${tmpdir}/kubeletconfig.json"
            emit_and_read() {
                emitKubeletActiveFlagsEvent
                cat "${EVENTS_LOGGING_DIR}"*.json
            }
            When call emit_and_read
            The output should include '"TaskName": "AKS.CSE.ensureKubelet.kubeletActiveFlags"'
            The output should include 'uses_config_file'
            The output should include 'cloud-provider'
        End
    End
End
