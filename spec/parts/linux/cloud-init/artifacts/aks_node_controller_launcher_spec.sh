#!/usr/bin/env shellspec

Describe 'aks-node-controller-launcher.sh'
    SCRIPT="./parts/linux/cloud-init/artifacts/aks-node-controller-launcher.sh"

    setup_wrapper_test() {
        TEST_DIR="${SHELLSPEC_WORKDIR}/aks-node-controller-launcher"
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
        # Feature-flag file is test-local and absent by default; tests that exercise the
        # source path create it explicitly.
        export FEATURES_PATH="${TEST_DIR}/enabled_features.sh"
    }

    cleanup_wrapper_test() {
        rm -rf "$TEST_DIR"
        unset BIN_PATH CONFIG_PATH NBC_CMD_PATH TEST_DIR BIN_DIR HOTFIX_JSON ENABLE_PROVISIONING_HOTFIX CHECK_HOTFIX_EXIT FEATURES_PATH
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

    # Stands in for the binary download-hotfix stages at "${BIN_PATH}-hotfix". It records to a
    # separate calls log so tests can prove which of the two binaries actually ran provision.
    create_staged_hotfix_binary() {
        cat >"${BIN_PATH}-hotfix" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >>"${TEST_DIR}/hotfix_calls"
exit 0
EOF
        chmod +x "${BIN_PATH}-hotfix"
    }

    # Mirrors the real chain: the VHD-baked binary itself stages the hotfix binary while handling
    # download-hotfix, so binary selection observes a file that did not exist when the wrapper started.
    create_staging_aks_node_controller() {
        cat >"${TEST_DIR}/hotfix-template" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >>"${TEST_DIR}/hotfix_calls"
exit 0
EOF

        cat >"$BIN_PATH" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >>"${TEST_DIR}/calls"
if [ "$1" = "download-hotfix" ]; then
    cp "${TEST_DIR}/hotfix-template" "${BIN_PATH}-hotfix"
    chmod +x "${BIN_PATH}-hotfix"
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

    It 'does not call check-hotfix when ENABLE_PROVISIONING_HOTFIX is unset'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller

        When run bash "$SCRIPT"
        The status should be success
        The output should not include "running check-hotfix"
        The path "${TEST_DIR}/calls" should be exist
        # Only provision should have been recorded; no check-hotfix line.
        calls=$(cat "${TEST_DIR}/calls")
        The variable calls should eq "provision"
    End

    It 'treats a non-true ENABLE_PROVISIONING_HOTFIX value as disabled'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        export ENABLE_PROVISIONING_HOTFIX="1"

        When run bash "$SCRIPT"
        The status should be success
        The output should not include "running check-hotfix"
        calls=$(cat "${TEST_DIR}/calls")
        The variable calls should eq "provision"
    End

    It 'runs check-hotfix before download-hotfix when ENABLE_PROVISIONING_HOTFIX is true'
        touch "$CONFIG_PATH" "$HOTFIX_JSON"
        create_recording_aks_node_controller
        export ENABLE_PROVISIONING_HOTFIX="true"

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

    # Fail-open also covers the backward-compat case where ENABLE_PROVISIONING_HOTFIX=true reaches
    # a node whose VHD-baked binary predates 2.1b: `check-hotfix` is an unknown subcommand
    # there and exits non-zero, which the wrapper tolerates so provisioning still proceeds.
    It 'proceeds to provision when check-hotfix fails (fail-open)'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        export ENABLE_PROVISIONING_HOTFIX="true"
        export CHECK_HOTFIX_EXIT="1"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "ANC check-hotfix failed; continuing (fail-open)"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        lastCall=$(tail -n 1 "${TEST_DIR}/calls")
        The variable firstCall should eq "check-hotfix"
        The variable lastCall should eq "provision"
    End

    # The feature-flag file is the on-node delivery channel: the boothook writes
    # ENABLE_PROVISIONING_HOTFIX=true into it, the wrapper parses it, and the existing gate fires -
    # no environment variable is set by systemd here.
    It 'parses enabled_features.sh and runs check-hotfix when the file sets the flag true'
        touch "$CONFIG_PATH" "$HOTFIX_JSON"
        create_recording_aks_node_controller
        printf 'ENABLE_PROVISIONING_HOTFIX=true\n' >"$FEATURES_PATH"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Reading feature flags from ${FEATURES_PATH}"
        The output should include "running check-hotfix"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        secondCall=$(sed -n '2p' "${TEST_DIR}/calls")
        thirdCall=$(sed -n '3p' "${TEST_DIR}/calls")
        The variable firstCall should eq "check-hotfix"
        The variable secondCall should eq "download-hotfix"
        The variable thirdCall should eq "provision"
    End

    # Regression: the parse loop must honor the final line even when the file has no trailing
    # newline. `read` returns non-zero at EOF but still populates the variables, so the
    # "|| [ -n "$_key" ]" guard keeps the last KEY=VALUE from being silently dropped.
    It 'parses the final line of enabled_features.sh without a trailing newline'
        touch "$CONFIG_PATH" "$HOTFIX_JSON"
        create_recording_aks_node_controller
        printf 'ENABLE_PROVISIONING_HOTFIX=true' >"$FEATURES_PATH"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Reading feature flags from ${FEATURES_PATH}"
        The output should include "running check-hotfix"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        The variable firstCall should eq "check-hotfix"
    End

    # Fail-open on an empty/miswritten feature file: with `set -u`, the first `read` hits EOF
    # immediately but still assigns an empty "$_key", so the "|| [ -n "$_key" ]" guard evaluates
    # cleanly (no unbound-variable abort) and the loop simply does not run. The wrapper must
    # provision normally and never invoke check-hotfix.
    It 'is a no-op when enabled_features.sh exists but is empty'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        : >"$FEATURES_PATH"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Reading feature flags from ${FEATURES_PATH}"
        The output should not include "running check-hotfix"
        calls=$(cat "${TEST_DIR}/calls")
        The variable calls should eq "provision"
    End

    It 'does not run check-hotfix when enabled_features.sh omits the flag'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        printf 'SOME_OTHER_FLAG=true\n' >"$FEATURES_PATH"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Reading feature flags from ${FEATURES_PATH}"
        The output should not include "running check-hotfix"
        calls=$(cat "${TEST_DIR}/calls")
        The variable calls should eq "provision"
    End

    # Security/fail-open: the file is PARSED, never executed. A hostile or malformed file
    # (arbitrary commands, exit, etc.) must not run or abort the wrapper; only KEY=VALUE lines
    # with valid identifier keys are honored. If the file were sourced, 'exit 7' would abort the
    # wrapper and 'touch .../pwned' would run - both are asserted NOT to happen here.
    It 'never executes enabled_features.sh contents (parses KEY=VALUE only)'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        {
            printf 'exit 7\n'
            printf 'touch "%s/pwned"\n' "$TEST_DIR"
            printf 'ENABLE_PROVISIONING_HOTFIX=true\n'
        } >"$FEATURES_PATH"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Reading feature flags from ${FEATURES_PATH}"
        The path "${TEST_DIR}/pwned" should not be exist
        The output should include "running check-hotfix"
        firstCall=$(sed -n '1p' "${TEST_DIR}/calls")
        lastCall=$(tail -n 1 "${TEST_DIR}/calls")
        The variable firstCall should eq "check-hotfix"
        The variable lastCall should eq "provision"
    End

    # Binary selection. The hotfix binary carries the embedded script payload, so "which binary runs
    # provision" decides whether that payload is ever applied. These cover the branch directly.
    It 'runs the staged hotfix binary for provision when one is present'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        create_staged_hotfix_binary

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Using hotfix binary: ${BIN_PATH}-hotfix"
        hotfixCall=$(tail -n 1 "${TEST_DIR}/hotfix_calls")
        The variable hotfixCall should eq "provision"
        # The VHD-baked binary must not have been invoked at all on this path.
        The path "${TEST_DIR}/calls" should not be exist
    End

    It 'falls back to the VHD-baked binary when the staged hotfix path is not executable'
        touch "$CONFIG_PATH"
        create_recording_aks_node_controller
        create_staged_hotfix_binary
        chmod -x "${BIN_PATH}-hotfix"

        When run bash "$SCRIPT"
        The status should be success
        The output should include "Using VHD-baked binary: ${BIN_PATH}"
        lastCall=$(tail -n 1 "${TEST_DIR}/calls")
        The variable lastCall should eq "provision"
        The path "${TEST_DIR}/hotfix_calls" should not be exist
    End

    # The full production seam for a version-only hotfix pointer: the baked binary handles
    # download-hotfix and stages the replacement, then provision runs on the STAGED binary. That
    # handoff is what lets the staged binary apply its compiled-in script payload; no scripts_version
    # is involved. Previously uncovered, because no test created "${BIN_PATH}-hotfix".
    It 'provisions with the binary staged by download-hotfix'
        touch "$CONFIG_PATH" "$HOTFIX_JSON"
        create_staging_aks_node_controller

        When run bash "$SCRIPT"
        The status should be success
        The output should include "ANC download-hotfix completed"
        The output should include "Using hotfix binary"
        bakedCalls=$(cat "${TEST_DIR}/calls")
        hotfixCall=$(tail -n 1 "${TEST_DIR}/hotfix_calls")
        # The baked binary only downloads; it must never be the one that provisions.
        The variable bakedCalls should eq "download-hotfix"
        The variable hotfixCall should eq "provision"
    End
End
