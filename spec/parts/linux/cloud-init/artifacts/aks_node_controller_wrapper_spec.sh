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
    }

    cleanup_wrapper_test() {
        rm -rf "$TEST_DIR"
        unset BIN_PATH CONFIG_PATH NBC_CMD_PATH TEST_DIR BIN_DIR
    }

    create_fake_aks_node_controller() {
        cat >"$BIN_PATH" <<'EOF'
#!/bin/sh
printf '%s\n' "$@" >"${TEST_DIR}/args"
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

    It 'runs the check-lps connectivity probe before provisioning'
        touch "$CONFIG_PATH"
        # Record every subcommand invocation so we can assert check-lps ran.
        cat >"$BIN_PATH" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >>"${TEST_DIR}/calls"
exit 0
EOF
        chmod +x "$BIN_PATH"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Running ANC check-lps pre-kubelet connectivity probe"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        secondCall=$(sed -n '2p' "${TEST_DIR}/calls")
        The variable firstCall should eq "check-lps"
        The variable secondCall should eq "provision"
    End

    It 'does not abort provisioning when check-lps fails'
        touch "$CONFIG_PATH"
        # Fake binary fails for check-lps but succeeds for provision.
        cat >"$BIN_PATH" <<'EOF'
#!/bin/sh
if [ "$1" = "check-lps" ]; then
    exit 1
fi
printf '%s\n' "$@" >"${TEST_DIR}/args"
exit 0
EOF
        chmod +x "$BIN_PATH"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "ANC check-lps returned non-zero (ignored)"
        The output should include "Launching aks-node-controller with config ${CONFIG_PATH}"
        firstArg=$(sed -n '1p' "${TEST_DIR}/args")
        The variable firstArg should eq "provision"
    End
End
