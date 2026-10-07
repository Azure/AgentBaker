#!/bin/bash

Describe 'explicit kubelet renderer omissions'
    Include ./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh

    setup() {
        TEST_DIR=$(mktemp -d)
        SERVICE_FILE="${TEST_DIR}/kubelet.service"
        DROP_IN_DIR="${TEST_DIR}/kubelet.service.d"
        CONFIG_FILE="${TEST_DIR}/kubeletconfig.json"
        MANAGED_DROP_IN="${DROP_IN_DIR}/11-kubelet-config-flags.conf"
        mkdir -p "${DROP_IN_DIR}"
        cp ./parts/linux/cloud-init/artifacts/kubelet.service "${SERVICE_FILE}"
        printf '[Service]\nEnvironment="KUBELET_CONFIG_FILE_FLAGS=--config %s"\n' "${CONFIG_FILE}" > "${DROP_IN_DIR}/10-componentconfig.conf"
        printf '[Service]\nEnvironment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd"\n' > "${DROP_IN_DIR}/10-cgroupv2.conf"
        printf '%s\n' '{"enableServer":true,"volumePluginDir":"/etc/kubernetes/volumeplugins","cgroupDriver":"systemd","runtimeRequestTimeout":"15m","containerRuntimeEndpoint":"unix:///run/containerd/containerd.sock"}' > "${CONFIG_FILE}"
        KUBELET_FLAGS_TO_OMIT=$(printf '%s' '["--enable-server","--volume-plugin-dir","--cgroup-driver","--runtime-request-timeout","--container-runtime-endpoint"]' | base64 -w0)
        KUBELET_CONFIG_FILE_ENABLED=true
        NEEDS_CGROUPV2=true
        KUBELET_FLAGS='--enable-server=false --volume-plugin-dir=/custom/plugins --runtime-request-timeout=2m --runtime-request-timeout=3m --node-ip=10.0.0.4'
        RUNTIME_CGROUPS=/system.slice/containerd.service
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    reconcile() {
        reconcileKubeletConfigFlags "${SERVICE_FILE}" "${DROP_IN_DIR}" "${CONFIG_FILE}" "${RUNTIME_CGROUPS}"
    }

    BeforeEach setup
    AfterEach cleanup

    It 'omits only the requested matching appenders and preserves the baked service and custom flags'
        When call reconcile
        The status should be success
        The contents of file "${MANAGED_DROP_IN}" should include 'ExecStart='
        The contents of file "${MANAGED_DROP_IN}" should not include '--enable-server'
        The contents of file "${MANAGED_DROP_IN}" should not include '--volume-plugin-dir'
        The contents of file "${MANAGED_DROP_IN}" should not include '--runtime-request-timeout'
        The contents of file "${MANAGED_DROP_IN}" should not include '--container-runtime-endpoint'
        The contents of file "${MANAGED_DROP_IN}" should include 'Environment="KUBELET_CGROUP_FLAGS="'
        The contents of file "${MANAGED_DROP_IN}" should include '--runtime-cgroups=/system.slice/containerd.service'
        The contents of file "${MANAGED_DROP_IN}" should include '$KUBELET_TLS_BOOTSTRAP_FLAGS'
        The contents of file "${MANAGED_DROP_IN}" should include '$KUBELET_CONFIG_FILE_FLAGS'
        The contents of file "${MANAGED_DROP_IN}" should include '$KUBELET_FLAGS'
        The contents of file "${SERVICE_FILE}" should equal "$(cat ./parts/linux/cloud-init/artifacts/kubelet.service)"
        The variable KUBELET_FLAGS should equal '--enable-server=false --volume-plugin-dir=/custom/plugins --runtime-request-timeout=2m --runtime-request-timeout=3m --node-ip=10.0.0.4'
    End

    Describe 'absent instruction leaves existing files byte-equivalent'
        Parameters
            1.31.0 true
            1.37.99 true
            1.38.0-alpha.1 true
            1.38.0-beta.0 true
            1.38.0-rc.1 true
            1.38.0 true
            1.38.1 true
            1.39.0 true
            1.38.0 false
        End
        It "does not infer omission from config or version $1, config $2"
            KUBERNETES_VERSION="$1"
            KUBELET_CONFIG_FILE_ENABLED="$2"
            unset KUBELET_FLAGS_TO_OMIT
            unchanged_files() {
                local before
                before=$(sha256sum "${SERVICE_FILE}" "${CONFIG_FILE}" "${DROP_IN_DIR}"/*)
                reconcile || return
                [ "${before}" = "$(sha256sum "${SERVICE_FILE}" "${CONFIG_FILE}" "${DROP_IN_DIR}"/*)" ]
            }
            When call unchanged_files
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
        End
    End

    Describe 'unsupported instructions'
        Parameters
            ''
            'W10='
            'invalid base64!'
            'e30='
            'WyIxIl0='
            'W251bGxd'
            'WyItLXVuZmFtaWxpYXItZmxhZyJd'
        End
        It "preserves the old output for $1"
            KUBELET_FLAGS_TO_OMIT="$1"
            When call reconcile
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
        End
    End

    It 'ignores an oversized encoded request'
        KUBELET_FLAGS_TO_OMIT=$(printf '%01025d' 0)
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'ignores a list with more than sixteen entries'
        KUBELET_FLAGS_TO_OMIT=$(jq -cn '[range(17) | "--enable-server"]' | base64 -w0)
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    Describe 'explicit replacement requirements'
        Parameters
            '--enable-server' '{}'
            '--enable-server' '{"enableServer":false}'
            '--enable-server' '{"enableServer":"true"}'
            '--volume-plugin-dir' '{"volumePluginDir":"/custom/plugins"}'
            '--volume-plugin-dir' '{"volumePluginDir":""}'
            '--cgroup-driver' '{"cgroupDriver":"cgroupfs"}'
            '--runtime-request-timeout' '{"runtimeRequestTimeout":"0s"}'
            '--runtime-request-timeout' '{"runtimeRequestTimeout":"15m0s"}'
            '--container-runtime-endpoint' '{"containerRuntimeEndpoint":"unix:///custom.sock"}'
        End
        It "retains $1 without its explicit matching replacement"
            KUBELET_FLAGS_TO_OMIT=$(printf '["%s"]' "$1" | base64 -w0)
            printf '%s\n' "$2" > "${CONFIG_FILE}"
            When call reconcile
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
        End
    End

    It 'preserves config-off even with explicit matching content and requests'
        KUBELET_CONFIG_FILE_ENABLED=false
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'does not omit anything without a loaded config drop-in'
        rm "${DROP_IN_DIR}/10-componentconfig.conf"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'does not omit anything with invalid config JSON'
        printf 'invalid' > "${CONFIG_FILE}"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    Describe 'custom configuration paths'
        Parameters
            '--config=/custom/kubelet.json'
            '--config /custom/kubelet.json'
            '--config-dir=/custom/config'
            '--config-dir /custom/config'
            '"--config=/custom/kubelet.json"'
            "--config	/custom/kubelet.json"
        End
        It "preserves output when the loaded configuration may differ: $1"
            KUBELET_FLAGS="$1"
            When call reconcile
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
        End
    End

    It 'allows a requested subset while retaining all other runtime arguments'
        KUBELET_FLAGS_TO_OMIT=$(printf '%s' '["--runtime-request-timeout","--unknown"]' | base64 -w0)
        When call reconcile
        The status should be success
        The contents of file "${MANAGED_DROP_IN}" should equal '[Service]
Environment="KUBELET_CONTAINERD_FLAGS=--container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=/system.slice/containerd.service"'
    End

    It 'accepts an unpadded list through the existing feature-file transport'
        KUBELET_FLAGS_TO_OMIT=$(printf '%s' '["--runtime-request-timeout"]' | base64 -w0 | tr -d '=')
        When call reconcile
        The status should be success
        The contents of file "${MANAGED_DROP_IN}" should not include '--runtime-request-timeout'
        The contents of file "${MANAGED_DROP_IN}" should include '--container-runtime-endpoint=unix:///run/containerd/containerd.sock'
    End

    It 'handles duplicate request names without changing remaining argument order'
        KUBELET_FLAGS_TO_OMIT=$(printf '%s' '["--enable-server","--enable-server"]' | base64 -w0)
        When call reconcile
        The status should be success
        The contents of file "${MANAGED_DROP_IN}" should not include '--enable-server'
        The contents of file "${MANAGED_DROP_IN}" should include '--volume-plugin-dir=/etc/kubernetes/volumeplugins'
    End

    It 'preserves hardened runtime-cgroups when refreshing a PIS node'
        RUNTIME_CGROUPS=/kubereserved.slice/containerd.service
        When call reconcile
        The status should be success
        The contents of file "${MANAGED_DROP_IN}" should include '--runtime-cgroups=/kubereserved.slice/containerd.service'
    End

    It 'does not override an unfamiliar service command'
        printf '[Service]\nExecStart=/custom/kubelet --enable-server\n' > "${SERVICE_FILE}"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'does not replace an existing custom ExecStart drop-in'
        printf '[Service]\nExecStart=\nExecStart=/custom/kubelet\n' > "${DROP_IN_DIR}/10-custom.conf"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'preserves cgroup arguments if the image has an unfamiliar drop-in'
        printf '[Service]\nEnvironment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd --custom=value"\n' > "${DROP_IN_DIR}/10-cgroupv2.conf"
        When call reconcile
        The status should be success
        The contents of file "${MANAGED_DROP_IN}" should not include 'KUBELET_CGROUP_FLAGS='
    End

    It 'does not infer cgroupv2 from config presence'
        NEEDS_CGROUPV2=false
        When call reconcile
        The status should be success
        The contents of file "${MANAGED_DROP_IN}" should not include 'KUBELET_CGROUP_FLAGS='
    End

    It 'is idempotent and restores the original output on rollback'
        reapply_and_rollback() {
            reconcile || return
            local first_output
            first_output=$(cat "${MANAGED_DROP_IN}")
            reconcile || return
            [ "${first_output}" = "$(cat "${MANAGED_DROP_IN}")" ] || return 1
            KUBELET_FLAGS_TO_OMIT=''
            reconcile
        }
        When call reapply_and_rollback
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
        The contents of file "${SERVICE_FILE}" should equal "$(cat ./parts/linux/cloud-init/artifacts/kubelet.service)"
    End

    It 'reconciles after the final runtime writer and before kubelet starts on real nodes'
        final_writer_order() {
            awk '/^ensureKubelet\(\)/ { inside = 1 } inside { print } inside && /^}/ { exit }' ./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh |
                grep -E 'Environment="KUBELET_CONTAINERD_FLAGS=|^[[:space:]]*reconcileKubeletConfigFlags |if ! systemctl daemon-reload|if ! systemctlEnableAndStartNoBlock kubelet'
        }
        When call final_writer_order
        The line 1 of output should include 'Environment="KUBELET_CONTAINERD_FLAGS='
        The line 2 of output should include 'reconcileKubeletConfigFlags '
        The line 3 of output should include 'systemctl daemon-reload'
        The line 4 of output should include 'systemctlEnableAndStartNoBlock kubelet'
        The lines of output should equal 4
    End
End
