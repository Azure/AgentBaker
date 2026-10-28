#!/usr/bin/env shellspec

Describe 'aks-networking-image-cache.sh'
    SCRIPT="./parts/linux/cloud-init/artifacts/aks-networking-image-cache.sh"

    setup_networking_image_cache_test() {
        TEST_DIR="${SHELLSPEC_WORKDIR}/aks-networking-image-cache"
        mkdir -p "$TEST_DIR/bin"
        export TEST_DIR
        export FEATURES_PATH="${TEST_DIR}/enabled_features.sh"
        export LAUNCHER_PATH="${TEST_DIR}/launcher"
        cat >"${TEST_DIR}/launcher" <<'EOF'
#!/bin/bash
printf '%s\n' "$@" >"${TEST_DIR}/args"
EOF
        chmod +x "${TEST_DIR}/launcher"
    }

    cleanup_networking_image_cache_test() {
        rm -rf "$TEST_DIR"
        unset TEST_DIR FEATURES_PATH LAUNCHER_PATH
    }

    BeforeEach setup_networking_image_cache_test
    AfterEach cleanup_networking_image_cache_test

    It 'is a no-op when the feature is disabled'
        When run bash "$SCRIPT"
        The status should be success
        The path "${TEST_DIR}/args" should not be exist
    End

    It 'invokes ANC prefetch when the feature is enabled'
        printf 'ENABLE_NETWORKING_IMAGE_CACHE=true\n' >"$FEATURES_PATH"

        When run bash "$SCRIPT"
        The status should be success
        args=$(cat "${TEST_DIR}/args")
        The variable args should eq "prepull-networking-images"
    End
End
