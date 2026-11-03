#!/bin/bash
# shellcheck disable=SC2034,SC2329

Describe 'cached NVIDIA container toolkit release notes'
    eval "$(sed -n '/^getCachedNvidiaContainerToolkitVersion()/,/^}/p' ./vhdbuilder/packer/install-dependencies.sh)"
    eval "$(sed -n '/^logCachedNvidiaContainerToolkitVersion()/,/^}/p' ./vhdbuilder/packer/install-dependencies.sh)"

    setup() {
        TEST_DIR="$(mktemp -d)"
        VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
        export TEST_CALLS="${TEST_DIR}/calls"
        export TEST_LAYER="${TEST_DIR}/toolkit.tar.gz" EMPTY_LAYER="${TEST_DIR}/empty.tar.gz"
        export CTR_STATUS=0 DIRECT_MANIFEST=false MISSING_TOOLKIT=false
        CPU_ARCH=amd64
        mkdir -p "${TEST_DIR}/archive/opt/gpu"
        touch "${TEST_DIR}/archive/opt/gpu/nvidia-container-toolkit_1.19.1-1_amd64.deb"
        tar -czf "$TEST_LAYER" -C "${TEST_DIR}/archive" opt/gpu
        tar -czf "$EMPTY_LAYER" -T /dev/null
    }

    cleanup() { rm -rf "$TEST_DIR"; }
    BeforeEach setup
    AfterEach cleanup

    Mock ctr
        printf '%s\n' "$*" >> "$TEST_CALLS"
        case "$*" in
            '-n k8s.io images list name==example.com/cuda:test-tag')
                printf 'REF TYPE DIGEST\nexample.com/cuda:test-tag oci '
                if [ "$DIRECT_MANIFEST" = true ]; then
                    echo sha256:manifest
                else
                    echo sha256:index
                fi
                ;;
            '-n k8s.io content get sha256:index')
                echo '{"manifests":[{"platform":{"os":"linux","architecture":"arm64"},"digest":"sha256:wrong-platform"},{"platform":{"os":"linux","architecture":"amd64"},"digest":"sha256:manifest"}]}'
                ;;
            '-n k8s.io content get sha256:manifest')
                echo '{"layers":[{"digest":"sha256:toolkit"},{"digest":"sha256:empty"}]}'
                ;;
            '-n k8s.io content get sha256:empty')
                cat "$EMPTY_LAYER"
                ;;
            '-n k8s.io content get sha256:toolkit')
                if [ "$MISSING_TOOLKIT" = true ]; then
                    cat "$EMPTY_LAYER"
                else
                    cat "$TEST_LAYER"
                fi
                exit "$CTR_STATUS"
                ;;
            *) echo "Unexpected containerd command: $*" >&2; exit 99 ;;
        esac
    End

    Describe 'cached image metadata'
        Parameters
            false
            true
        End

        It "reads the selected platform and compressed layers without unpacking (direct manifest=$1)"
            DIRECT_MANIFEST="$1"
            When call logCachedNvidiaContainerToolkitVersion example.com/cuda:test-tag
            The status should be success
            The contents of file "$VHD_LOGS_FILEPATH" should include 'nvidia-container-toolkit version 1.19.1-1 (cached in CUDA driver image; installed at node provisioning time)'
            The contents of file "$TEST_CALLS" should include '-n k8s.io content get sha256:toolkit'
            The contents of file "$TEST_CALLS" should not include ' run '
            The contents of file "$TEST_CALLS" should not include 'mount'
        End
    End

    Describe 'lookup failures'
        Parameters
            42
            124
        End

        It "warns without failing the build when the query exits $1"
            CTR_STATUS="$1"
            When call logCachedNvidiaContainerToolkitVersion example.com/cuda:test-tag
            The status should be success
            The error should include 'Warning: unable to read NVIDIA container toolkit version'
            The contents of file "$TEST_CALLS" should include '-n k8s.io content get sha256:toolkit'
            The path "$VHD_LOGS_FILEPATH" should not be exist
        End
    End

    It 'warns instead of inventing a version when the toolkit is absent'
        MISSING_TOOLKIT=true
        When call logCachedNvidiaContainerToolkitVersion example.com/cuda:test-tag
        The status should be success
        The error should include 'Warning: unable to read NVIDIA container toolkit version'
        The contents of file "$TEST_CALLS" should include '-n k8s.io content get sha256:toolkit'
        The path "$VHD_LOGS_FILEPATH" should not be exist
    End
End
