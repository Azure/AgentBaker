#!/usr/bin/env shellspec

Describe 'aks-node-controller-wrapper.sh'
    SCRIPT="./parts/linux/cloud-init/artifacts/aks-node-controller-wrapper.sh"

    setup_wrapper_test() {
        TEST_DIR="${SHELLSPEC_WORKDIR}/aks-node-controller-wrapper"
        BIN_DIR="${TEST_DIR}/bin"
        mkdir -p "$BIN_DIR"

        cat >"${BIN_DIR}/hostname" <<'EOF'
#!/bin/sh
printf 'test-host\n'
EOF
        chmod +x "${BIN_DIR}/hostname"

        cat >"${BIN_DIR}/cat" <<'EOF'
#!/bin/sh
if [ "$1" = "/etc/hostname" ]; then
    printf 'test-host\n'
else
    /bin/cat "$@"
fi
EOF
        chmod +x "${BIN_DIR}/cat"

        cat >"${BIN_DIR}/logger" <<'EOF'
#!/bin/sh
exit 0
EOF
        chmod +x "${BIN_DIR}/logger"

        export PATH="${BIN_DIR}:$PATH"
        export TEST_DIR
        export BIN_PATH="${TEST_DIR}/aks-node-controller"
        export CONFIG_PATH="${TEST_DIR}/aks-node-controller-config.json"
        export NBC_CMD_PATH="${TEST_DIR}/aks-node-controller-nbc-cmd.sh"
        # Point hotfix pointer at a test-local path (absent by default) so tests never
        # touch the production /opt/azure path and can control the download-hotfix branch.
        export HOTFIX_JSON="${TEST_DIR}/aks-node-controller-hotfix.json"
    }

    cleanup_wrapper_test() {
        rm -rf "$TEST_DIR"
        unset BIN_PATH CONFIG_PATH NBC_CMD_PATH TEST_DIR BIN_DIR HOTFIX_JSON CHECK_HOTFIX_EXIT
    }

    create_fake_aks_node_controller() {
        cat >"$BIN_PATH" <<'EOF'
#!/bin/sh
printf '%s\n' "$@" >"${TEST_DIR}/args"
exit 0
EOF
        chmod +x "$BIN_PATH"
    }

    # Records each subcommand (first arg) on its own line in calls log so ordering across
    # multiple invocations (check-hotfix vs download-hotfix vs provision) is observable.
    # CHECK_HOTFIX_EXIT controls the exit code of the check-hotfix invocation only.
    create_recording_aks_node_controller() {
        cat >"$BIN_PATH" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >>"${TEST_DIR}/calls"
if [ "$1" = "check-hotfix" ]; then
    exit "${CHECK_HOTFIX_EXIT:-0}"
fi
exit 0
EOF
        chmod +x "$BIN_PATH"
    }

    BeforeEach setup_wrapper_test
    AfterEach cleanup_wrapper_test

    It 'exits successfully without invoking aks-node-controller when config and nbc cmd are absent'
        When run bash "$SCRIPT"
        The status should be success
        The output should include "Gracefully exit aks-node-controller without provision config or nbc cmd"
        The output should not include "Spawned aks-node-controller"
    End

    It 'passes both provision config and nbc cmd when both files are present'
        touch "$CONFIG_PATH" "$NBC_CMD_PATH"
        create_fake_aks_node_controller

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Launching aks-node-controller with config ${CONFIG_PATH}"
        The output should include "Launching aks-node-controller with nbc cmd ${NBC_CMD_PATH}"
        firstArg=$(sed -n '1p' "${TEST_DIR}/args")
        secondArg=$(sed -n '2p' "${TEST_DIR}/args")
        thirdArg=$(sed -n '3p' "${TEST_DIR}/args")
        The variable firstArg should eq "provision"
        The variable secondArg should eq "--provision-config=${CONFIG_PATH}"
        The variable thirdArg should eq "--nbc-cmd=${NBC_CMD_PATH}"
    End

    It 'passes only provision config when nbc cmd is absent'
        touch "$CONFIG_PATH"
        create_fake_aks_node_controller

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Launching aks-node-controller with config ${CONFIG_PATH}"
        The output should not include "Launching aks-node-controller with nbc cmd"
        firstArg=$(sed -n '1p' "${TEST_DIR}/args")
        secondArg=$(sed -n '2p' "${TEST_DIR}/args")
        thirdArg=$(sed -n '3p' "${TEST_DIR}/args")
        The variable firstArg should eq "provision"
        The variable secondArg should eq "--provision-config=${CONFIG_PATH}"
        The variable thirdArg should eq ""
    End

    It 'passes only nbc cmd when provision config is absent'
        touch "$NBC_CMD_PATH"
        create_fake_aks_node_controller

        When run bash "$SCRIPT"
        The status should be success
        The output should not include "Launching aks-node-controller with config"
        The output should include "Launching aks-node-controller with nbc cmd ${NBC_CMD_PATH}"
        firstArg=$(sed -n '1p' "${TEST_DIR}/args")
        secondArg=$(sed -n '2p' "${TEST_DIR}/args")
        thirdArg=$(sed -n '3p' "${TEST_DIR}/args")
        The variable firstArg should eq "provision"
        The variable secondArg should eq "--nbc-cmd=${NBC_CMD_PATH}"
        The variable thirdArg should eq ""
    End

    # check-hotfix is now called UNCONDITIONALLY (2.1d). The wrapper no longer reads the
    # ENABLE_PROVISIONING_HOTFIX env gate; the aks-node-controller binary self-gates on the
    # enable_provisioning_hotfix AKSNodeConfig contract field (single source of truth) and
    # no-ops when the feature is off. So the wrapper always invokes it and stays fail-open.
    It 'always runs check-hotfix before download-hotfix and provision'
        touch "$CONFIG_PATH" "$HOTFIX_JSON"
        create_recording_aks_node_controller

        When run bash "$SCRIPT"
        The status should be success
        The output should include "running check-hotfix"
        The output should include "ANC check-hotfix completed"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        secondCall=$(sed -n '2p' "${TEST_DIR}/calls")
        thirdCall=$(sed -n '3p' "${TEST_DIR}/calls")
        The variable firstCall should eq "check-hotfix"
        The variable secondCall should eq "download-hotfix"
        The variable thirdCall should eq "provision"
    End

    It 'runs check-hotfix even with no hotfix pointer present (binary self-gates)'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller

        When run bash "$SCRIPT"
        The status should be success
        The output should include "running check-hotfix"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        lastCall=$(tail -n 1 "${TEST_DIR}/calls")
        The variable firstCall should eq "check-hotfix"
        The variable lastCall should eq "provision"
    End

    # Fail-open also covers the backward-compat case where check-hotfix reaches a node whose
    # VHD-baked binary predates 2.1b: `check-hotfix` is an unknown subcommand there and exits
    # non-zero, which the wrapper tolerates so provisioning still proceeds.
    It 'proceeds to provision when check-hotfix fails (fail-open)'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        export CHECK_HOTFIX_EXIT="1"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "ANC check-hotfix failed; continuing (fail-open)"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        lastCall=$(tail -n 1 "${TEST_DIR}/calls")
        The variable firstCall should eq "check-hotfix"
        The variable lastCall should eq "provision"
    End
End
