#!/usr/bin/env shellspec

Describe 'stamp-kata-containerd-config systemd unit'
    SERVICE_UNIT='./parts/linux/cloud-init/artifacts/stamp-kata-containerd-config.service'
    STAMP_SCRIPT='./parts/linux/cloud-init/artifacts/stamp-kata-containerd-config.sh'

    It 'does not bind the stamp service lifecycle to containerd'
        When call cat "$SERVICE_UNIT"
        The status should be success
        The output should not include 'After=containerd.service'
        The output should not include 'Requires=containerd.service'
    End

    It 'restarts containerd explicitly from the stamp script'
        When call grep -F 'systemctl restart containerd' "$STAMP_SCRIPT"
        The status should be success
        The output should include 'systemctl restart containerd'
    End
End

Describe 'stampKataContainerdConfig'
    Include './parts/linux/cloud-init/artifacts/stamp-kata-containerd-config.sh'

    setup() {
        TEST_ROOT="$(mktemp -d)"
        MARKER_FILE="${TEST_ROOT}/stamped"
        CONFIG_SRC="${TEST_ROOT}/source.toml"
        CONFIG_DEST="${TEST_ROOT}/config.toml"
        SYSTEMCTL_STATUS=0
        echo 'version = 2' > "$CONFIG_SRC"
    }

    cleanup() {
        rm -rf "$TEST_ROOT"
    }

    systemctl() {
        echo "systemctl $*"
        return "$SYSTEMCTL_STATUS"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'stamps once and records success after containerd restarts'
        When call stampKataContainerdConfig

        The status should be success
        The path "$CONFIG_DEST" should be file
        The path "$MARKER_FILE" should be file
        The output should include 'systemctl restart containerd'
        The output should include 'kata containerd config stamped successfully'
    End

    It 'does not mark a failed containerd restart as stamped'
        SYSTEMCTL_STATUS=1

        When call stampKataContainerdConfig

        The status should be failure
        The path "$CONFIG_DEST" should be file
        The path "$MARKER_FILE" should not be exist
        The output should include 'systemctl restart containerd'
        The stderr should include 'failed to restart containerd'
    End

    It 'skips stamping when the success marker already exists'
        touch "$MARKER_FILE"

        When call stampKataContainerdConfig

        The status should be success
        The path "$CONFIG_DEST" should not be exist
        The output should include 'kata containerd config already stamped'
        The output should not include 'systemctl restart containerd'
    End
End