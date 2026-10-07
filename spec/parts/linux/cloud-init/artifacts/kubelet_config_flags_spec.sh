#!/bin/bash

Describe 'explicit kubelet renderer omissions'
    Include ./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh

    setup() {
        TEST_DIR=$(mktemp -d)
        SERVICE_FILE="${TEST_DIR}/etc/systemd/system/kubelet.service"
        DROP_IN_DIR="${SERVICE_FILE}.d"
        CONFIG_FILE="${TEST_DIR}/etc/default/kubeletconfig.json"
        DEFAULT_FILE="${TEST_DIR}/etc/default/kubelet"
        MANAGED_DROP_IN="${DROP_IN_DIR}/11-kubelet-config-flags.conf"
        mkdir -p "${DROP_IN_DIR}" "${TEST_DIR}/etc/default"
        cp ./parts/linux/cloud-init/artifacts/kubelet.service "${SERVICE_FILE}"
        printf '[Service]\nEnvironment="KUBELET_CONFIG_FILE_FLAGS=--config /etc/default/kubeletconfig.json"\n' > "${DROP_IN_DIR}/10-componentconfig.conf"
        printf '[Service]\nEnvironment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd"\n' > "${DROP_IN_DIR}/10-cgroupv2.conf"
        printf '%s\n' '{"enableServer":true,"volumePluginDir":"/etc/kubernetes/volumeplugins","cgroupDriver":"systemd","runtimeRequestTimeout":"15m","containerRuntimeEndpoint":"unix:///run/containerd/containerd.sock"}' > "${CONFIG_FILE}"
        KUBELET_FLAGS_TO_OMIT=$(printf '%s' '["--enable-server","--volume-plugin-dir","--cgroup-driver","--runtime-request-timeout","--container-runtime-endpoint"]' | base64 -w0)
        KUBELET_CONFIG_FILE_ENABLED=true
        NEEDS_CGROUPV2=true
        KUBELET_FLAGS='--enable-server=false --volume-plugin-dir=/custom/plugins --runtime-request-timeout=2m --runtime-request-timeout=3m --node-ip=10.0.0.4'
        RUNTIME_CGROUPS=/system.slice/containerd.service
        printf 'KUBELET_FLAGS=%s\nKUBELET_REGISTER_SCHEDULABLE=true\nNETWORK_POLICY=\nKUBELET_IMAGE=\nKUBELET_NODE_LABELS=kubernetes.azure.com/test=true\n' "${KUBELET_FLAGS}" > "${DEFAULT_FILE}"
        write_runtime_drop_in
    }

    write_runtime_drop_in() {
        printf '[Service]\nEnvironment="KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=%s"\n' "${RUNTIME_CGROUPS}" > "${DROP_IN_DIR}/10-containerd-base-flag.conf"
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    reconcile() {
        reconcileKubeletConfigFlags "${RUNTIME_CGROUPS}" "${TEST_DIR}"
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
        The contents of file "${MANAGED_DROP_IN}" should include '[Service]
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
        write_runtime_drop_in
        printf '[Unit]\nWants=kubereserved.slice\nAfter=kubereserved.slice\n\n[Service]\nSlice=kubereserved.slice\n' > "${DROP_IN_DIR}/10-kubereserved-slice.conf"
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

    It 'retains all arguments if the image has an unfamiliar cgroup drop-in'
        printf '[Service]\nEnvironment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd --custom=value"\n' > "${DROP_IN_DIR}/10-cgroupv2.conf"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
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

    Describe 'unowned path collisions'
        Parameters
            absent
            empty
            config-off
            requested
        End
        It "preserves an existing user file when $1"
            case "$1" in
                absent) unset KUBELET_FLAGS_TO_OMIT ;;
                empty) KUBELET_FLAGS_TO_OMIT='' ;;
                config-off) KUBELET_CONFIG_FILE_ENABLED=false ;;
            esac
            printf '[Service]\nEnvironment="CUSTOM=preserve-me"\n' > "${MANAGED_DROP_IN}"
            When call reconcile
            The status should be success
            The contents of file "${MANAGED_DROP_IN}" should equal '[Service]
Environment="CUSTOM=preserve-me"'
        End
    End

    Describe 'edited owned files'
        Parameters
            absent
            requested
        End
        It "preserves a generated file edited by the user when $1"
            reconcile
            printf 'Environment="CUSTOM=preserve-me"\n' >> "${MANAGED_DROP_IN}"
            local edited_digest
            edited_digest=$(sha256sum "${MANAGED_DROP_IN}")
            [ "$1" != absent ] || unset KUBELET_FLAGS_TO_OMIT
            When call reconcile
            The status should be success
            The contents of file "${MANAGED_DROP_IN}" should include 'CUSTOM=preserve-me'
            The value "$(sha256sum "${MANAGED_DROP_IN}")" should equal "${edited_digest}"
        End
    End

    It 'does not follow a symlink even to a previously owned file'
        reconcile
        mv "${MANAGED_DROP_IN}" "${TEST_DIR}/saved.conf"
        ln -s "${TEST_DIR}/saved.conf" "${MANAGED_DROP_IN}"
        unset KUBELET_FLAGS_TO_OMIT
        When call reconcile
        The status should be success
        The path "${MANAGED_DROP_IN}" should be symlink
        The file "${TEST_DIR}/saved.conf" should be exist
    End

    It 'keeps the prior owned file intact if atomic replacement fails'
        reconcile
        local original_digest
        original_digest=$(sha256sum "${MANAGED_DROP_IN}")
        KUBELET_FLAGS_TO_OMIT=$(printf '%s' '["--enable-server"]' | base64 -w0)
        mv() { return 1; }
        When call reconcile
        The status should be failure
        The value "$(sha256sum "${MANAGED_DROP_IN}")" should equal "${original_digest}"
        The value "$(find "${DROP_IN_DIR}" -name '11-kubelet-config-flags.conf.*' | wc -l)" should equal 0
    End

    Describe 'competing systemd environment'
        Parameters
            'Environment="KUBELET_CONFIG_FILE_FLAGS=--config /custom.json"'
            'Environment="KUBELET_FLAGS=--config /custom.json"'
            'Environment="KUBELET_CONTAINERD_FLAGS=--runtime-cgroups=/custom.slice/containerd.service"'
            'Environment="KUBELET_CGROUP_FLAGS=--cgroup-driver=cgroupfs"'
            'Environment="KUBELET_TLS_BOOTSTRAP_FLAGS=--config /custom.json"'
            'Environment="KUBELET_CONTAINER_RUNTIME_FLAG=--config /custom.json"'
            'EnvironmentFile=/custom/environment'
            'EnvironmentFile=-/missing/environment'
            'EnvironmentFile='
            'Environment='
            'UnsetEnvironment=KUBELET_CONFIG_FILE_FLAGS'
            'PassEnvironment=KUBELET_CONFIG_FILE_FLAGS'
            ' Environment = "KUBELET_CONFIG_FILE_FLAGS=--config /custom.json"'
            'ExecStart = /custom/kubelet'
            'RootDirectory=/custom-root'
            'BindReadOnlyPaths=/custom/config.json:/etc/default/kubeletconfig.json'
            'ExecStartPre=/custom/change-config'
            'MemoryMax=1G'
        End
        It "retains all flags for $1 without changing the custom file"
            printf '[Service]\n%s\n' "$1" > "${DROP_IN_DIR}/20-custom.conf"
            local custom_digest
            custom_digest=$(sha256sum "${DROP_IN_DIR}/20-custom.conf")
            When call reconcile
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
            The value "$(sha256sum "${DROP_IN_DIR}/20-custom.conf")" should equal "${custom_digest}"
        End
    End

    It 'removes only its previous override when a later custom environment appears'
        reconcile
        printf '[Service]\nEnvironment="KUBELET_CONFIG_FILE_FLAGS=--config /custom.json"\n' > "${DROP_IN_DIR}/20-custom.conf"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
        The file "${DROP_IN_DIR}/20-custom.conf" should be exist
    End

    Describe 'other systemd drop-in directories'
        Parameters
            /run/systemd/system/kubelet.service.d
            /usr/lib/systemd/system/kubelet.service.d
            /etc/systemd/system/service.d
        End
        It "retains flags for an environment override in $1"
            mkdir -p "${TEST_DIR}$1"
            printf '[Service]\nEnvironment="KUBELET_CONFIG_FILE_FLAGS=--config /custom.json"\n' > "${TEST_DIR}$1/20-custom.conf"
            When call reconcile
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
        End
    End

    Describe 'persisted base EnvironmentFile, not the CSE environment'
        Parameters
            'KUBELET_FLAGS=--config /custom.json'
            'KUBELET_FLAGS=--config-dir=/custom'
            'KUBELET_CONFIG_FILE_FLAGS=--config /custom.json'
            'KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=2m'
            'KUBELET_CGROUP_FLAGS=--cgroup-driver=cgroupfs'
            'KUBELET_TLS_BOOTSTRAP_FLAGS=--config /custom.json'
            'KUBELET_CONTAINER_RUNTIME_FLAG=--config /custom.json'
            'KUBELET_FLAGS="--node-ip=10.0.0.4"'
            "KUBELET_FLAGS='--node-ip=10.0.0.4'"
            'KUBELET_FLAGS=--node-ip=10.0.0.4\'
        End
        It "retains all flags for persisted $1 even when CSE flags match"
            printf '%s\n' "$1" > "${DEFAULT_FILE}"
            When call reconcile
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
        End
    End

    It 'preserves supported persisted custom flags that differ from CSE flags'
        printf 'KUBELET_FLAGS=--runtime-request-timeout=2m --runtime-request-timeout=3m\n' > "${DEFAULT_FILE}"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should be exist
        The contents of file "${DEFAULT_FILE}" should equal 'KUBELET_FLAGS=--runtime-request-timeout=2m --runtime-request-timeout=3m'
    End

    It 'retains flags if the base environment file is missing'
        rm "${DEFAULT_FILE}"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'retains flags for an altered base service EnvironmentFile'
        sed -i 's|EnvironmentFile=/etc/default/kubelet|EnvironmentFile=/custom/environment|' "${SERVICE_FILE}"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'retains flags for an unrecognized persisted runtime-cgroup value'
        RUNTIME_CGROUPS=/kubereserved.slice/containerd.service
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'retains flags for the HTTP proxy EnvironmentFile surface'
        printf '[Service]\nEnvironmentFile=/etc/environment\n' > "${DROP_IN_DIR}/10-httpproxy.conf"
        printf 'HTTPS_PROXY=http://proxy.example:8080\nKUBELET_CONFIG_FILE_FLAGS=--config /custom.json\n' > "${TEST_DIR}/etc/environment"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should not be exist
    End

    It 'does not confuse the separate secure-TLS service environment with kubelet'
        mkdir -p "${TEST_DIR}/etc/systemd/system/secure-tls-bootstrap.service.d"
        printf '[Service]\nEnvironmentFile=/etc/default/secure-tls-bootstrap\n' > "${TEST_DIR}/etc/systemd/system/secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
        printf 'BOOTSTRAP_FLAGS=--apiserver-fqdn=example.test\n' > "${TEST_DIR}/etc/default/secure-tls-bootstrap"
        printf '[Service]\nEnvironment="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"\n' > "${DROP_IN_DIR}/10-tlsbootstrap.conf"
        printf '[Service]\nEnvironment="PRIMARY_NIC_IP=10.0.0.4"\nEnvironment="ENABLE_IMDS_RESTRICTION=true"\nEnvironment="INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE=false"\n' > "${DROP_IN_DIR}/10-ensure-imds-restriction.conf"
        printf '[Service]\nEnvironment="CREDENTIAL_VALIDATION_KUBE_CA_FILE=/etc/kubernetes/certs/ca.crt"\nEnvironment="CREDENTIAL_VALIDATION_APISERVER_URL=https://example.test:443"\n' > "${DROP_IN_DIR}/10-credential-validation.conf"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should be exist
    End

    It 'accepts only the shipped watchdog and bindmount dependency bodies'
        printf '[Service]\nWatchdogSec=60s\n' > "${DROP_IN_DIR}/10-watchdog.conf"
        printf '[Unit]\nRequires=bind-mount.service\nAfter=bind-mount.service\n' > "${DROP_IN_DIR}/10-bindmount.conf"
        When call reconcile
        The status should be success
        The file "${MANAGED_DROP_IN}" should be exist
    End

    Describe 'known names do not authorize extra directives'
        Parameters
            10-watchdog.conf
            10-bindmount.conf
            10-kubereserved-slice.conf
            10-ensure-imds-restriction.conf
            10-credential-validation.conf
            10-componentconfig.conf
            10-containerd-base-flag.conf
            10-cgroupv2.conf
            10-tlsbootstrap.conf
        End
        It "retains all flags if $1 contains a custom pre-start command"
            printf '[Service]\nExecStartPre=/custom/change-config\n' >> "${DROP_IN_DIR}/$1"
            When call reconcile
            The status should be success
            The file "${MANAGED_DROP_IN}" should not be exist
        End
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
