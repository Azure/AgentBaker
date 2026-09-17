#!/usr/bin/env shellspec

Describe 'aks-node-controller-hotfix.sh'
    HOTFIX_SCRIPT="./parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.sh"

    setup_wrapper_test() {
        TEST_DIR="${SHELLSPEC_WORKDIR}/aks-node-controller-hotfix"
        BIN_DIR="${TEST_DIR}/bin"
        mkdir -p "$BIN_DIR"

        cat >"${BIN_DIR}/logger" <<'EOF'
#!/bin/sh
exit 0
EOF
        chmod +x "${BIN_DIR}/logger"

        export PATH="${BIN_DIR}:$PATH"
        export TEST_DIR
        export BIN_PATH="${TEST_DIR}/aks-node-controller"
        # Point hotfix pointer at a test-local path (absent by default) so tests never
        # touch the production /opt/azure path and can control the download-hotfix branch.
        export HOTFIX_JSON="${TEST_DIR}/aks-node-controller-hotfix.json"
        # Feature-flag file is test-local and absent by default; tests that exercise the
        # source path create it explicitly.
        export FEATURES_PATH="${TEST_DIR}/enabled_features.sh"
    }

    cleanup_wrapper_test() {
        rm -rf "$TEST_DIR"
        unset BIN_PATH TEST_DIR BIN_DIR HOTFIX_JSON ENABLE_PROVISIONING_HOTFIX CHECK_HOTFIX_EXIT FEATURES_PATH ANC_HOTFIX_SELECTED_BIN
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

    # Stands in for a staged hotfix binary whose embedded payload fails to apply. The flow is
    # fail-open there, so selection must still stand.
    create_failing_apply_hotfix_binary() {
        cat >"${BIN_PATH}-hotfix" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >>"${TEST_DIR}/hotfix_calls"
if [ "$1" = "apply-embedded-hotfix" ]; then
    exit 1
fi
exit 0
EOF
        chmod +x "${BIN_PATH}-hotfix"
    }

    # Runs the extracted flow directly, without the provision wrapper around it, and prints the
    # binary it selected. This is the seam the extraction bought: these cases assert what the
    # flow itself does, so they do not need a config/nbc-cmd file or a provision invocation.
    run_hotfix_flow() {
        bash -c 'source "$1"; anc_run_hotfix_flow "$2" "$3" "$4" "$5"; printf "%s\n" "$ANC_HOTFIX_SELECTED_BIN"' \
            _ "$HOTFIX_SCRIPT" "$BIN_PATH" "${BIN_PATH}-hotfix" "$HOTFIX_JSON" "$FEATURES_PATH"
    }

    BeforeEach setup_wrapper_test
    AfterEach cleanup_wrapper_test

    It 'can run the extracted hotfix flow without invoking provision'
        touch "$HOTFIX_JSON"
        create_recording_aks_node_controller
        create_staged_hotfix_binary
        printf 'ENABLE_PROVISIONING_HOTFIX=true\n' >"$FEATURES_PATH"

        When run run_hotfix_flow
        The status should be success
        The output should include "${BIN_PATH}-hotfix"
        calls=$(cat "${TEST_DIR}/calls")
        hotfixCalls=$(cat "${TEST_DIR}/hotfix_calls")
        The variable calls should eq "$(printf 'check-hotfix\ndownload-hotfix')"
        The variable hotfixCalls should eq "apply-embedded-hotfix"
    End

    # check-hotfix and download-hotfix are gated independently: the flag drives the former, the
    # pointer file's existence drives the latter. With the flag on but no pointer on disk (an LPS
    # that has nothing published, the steady state), only check-hotfix may run.
    It 'runs check-hotfix but not download-hotfix when the flag is on and no pointer exists'
        create_recording_aks_node_controller
        printf 'ENABLE_PROVISIONING_HOTFIX=true\n' >"$FEATURES_PATH"

        When run run_hotfix_flow
        The status should be success
        The output should include "running check-hotfix"
        The output should not include "running download-hotfix"
        The output should include "Using VHD-baked binary: ${BIN_PATH}"
        calls=$(cat "${TEST_DIR}/calls")
        The variable calls should eq "check-hotfix"
        The path "${TEST_DIR}/hotfix_calls" should not be exist
    End

    # The mirror of the case above: no flag, but a pointer left on disk (e.g. staged by a previous
    # boot). download-hotfix is reachable without the feature gate, which is what keeps the
    # cold-start/customdata pointer path working while the gate is still off by default.
    It 'runs download-hotfix without the feature flag when a pointer exists'
        touch "$HOTFIX_JSON"
        create_recording_aks_node_controller

        When run run_hotfix_flow
        The status should be success
        The output should not include "running check-hotfix"
        The output should include "Found ANC hotfix config"
        calls=$(cat "${TEST_DIR}/calls")
        The variable calls should eq "download-hotfix"
    End

    # apply-embedded-hotfix is fail-open: a payload that cannot be applied must not unselect the
    # hotfix binary, because it is still the newer ANC and provisioning has to proceed on it.
    It 'keeps the hotfix binary selected when apply-embedded-hotfix fails'
        create_recording_aks_node_controller
        create_failing_apply_hotfix_binary

        When run run_hotfix_flow
        The status should be success
        The output should include "ANC apply-embedded-hotfix failed"
        The output should include "Using hotfix binary: ${BIN_PATH}-hotfix"
        # The selection is the flow's only output contract, and it must survive the failure.
        The output should include "${BIN_PATH}-hotfix"
        hotfixCalls=$(cat "${TEST_DIR}/hotfix_calls")
        The variable hotfixCalls should eq "apply-embedded-hotfix"
    End

    # Only the literal "true" arms the gate, and the feature file is the delivery channel for it.
    # This pins the parse+gate pair at the flow level, where no provision run can mask it.
    It 'treats a non-true flag in the feature file as disabled'
        create_recording_aks_node_controller
        printf 'ENABLE_PROVISIONING_HOTFIX=TRUE\n' >"$FEATURES_PATH"

        When run run_hotfix_flow
        The status should be success
        The output should include "Reading feature flags from ${FEATURES_PATH}"
        The output should not include "running check-hotfix"
        The output should include "Using VHD-baked binary: ${BIN_PATH}"
        The path "${TEST_DIR}/calls" should not be exist
    End

    # Multiple flags in one file: the loop must keep parsing past an unrelated key, and values
    # containing "=" must survive intact rather than being truncated at the first separator.
    It 'parses every KEY=VALUE line in the feature file'
        create_recording_aks_node_controller
        {
            printf 'SOME_OTHER_FLAG=a=b\n'
            printf '# a comment\n'
            printf '\n'
            printf 'ENABLE_PROVISIONING_HOTFIX=true\n'
        } >"$FEATURES_PATH"

        When run run_hotfix_flow
        The status should be success
        The output should include "running check-hotfix"
        calls=$(cat "${TEST_DIR}/calls")
        The variable calls should eq "check-hotfix"
    End
End
