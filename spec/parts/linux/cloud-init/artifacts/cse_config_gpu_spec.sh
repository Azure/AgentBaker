#!/bin/bash

Describe 'cse_config_gpu.sh'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    CSE_CONFIG_ADDONS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_addons.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'logGPUDriverPrebakeReadiness'
        It 'reports marker_present=false when no prebake marker exists'
            GPU_DKMS_MARKER_FILE="$(mktemp)"; rm -f "${GPU_DKMS_MARKER_FILE}"
            NVIDIA_GPU_DRIVER_TYPE="cuda"
            When call logGPUDriverPrebakeReadiness
            The output should include "AKS_GPU_PREBAKE event=managed_gpu"
            The output should include "marker_present=false"
            The output should include "driver_kind_match=false"
        End

        It 'reports marker_present=true driver_kind_match=true when the marker matches the node driver kind'
            marker="$(mktemp)"
            printf 'driver_kind=cuda\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="cuda"
            When call logGPUDriverPrebakeReadiness
            The output should include "marker_present=true"
            The output should include "driver_kind_match=true"
            rm -f "$marker"
        End

        It 'matches a cuda marker for a cuda-lts (R580 LTS) node: driver-type maps to the aks-gpu driver_kind'
            marker="$(mktemp)"
            printf 'driver_kind=cuda\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="cuda-lts"
            When call logGPUDriverPrebakeReadiness
            The output should include "marker_present=true"
            The output should include "driver_kind_match=true"
            The output should include "driver_type=cuda-lts"
            rm -f "$marker"
        End

        It 'reports driver_kind_match=false when a CUDA marker is on a GRID node (not skip-ready)'
            marker="$(mktemp)"
            printf 'driver_kind=cuda\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            When call logGPUDriverPrebakeReadiness
            The output should include "marker_present=true"
            The output should include "driver_kind_match=false"
            rm -f "$marker"
        End

        It 'does not false-positive when the marker lacks driver_kind and the driver type is unset (both empty)'
            marker="$(mktemp)"
            printf 'kernel=5.15.0-1114-azure\n' > "$marker"   # no driver_kind= line
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE=""
            When call logGPUDriverPrebakeReadiness
            The output should include "marker_present=true"
            The output should include "driver_kind_match=false"
            rm -f "$marker"
        End
    End
    Describe 'ensureArtifactStreaming'
        # ensureArtifactStreaming enables the acr-mirror/overlaybd services and then
        # runs the version-appropriate enablement path:
        #   - acr-mirror 1.0.0+ -> setup.sh aks
        #   - older packages    -> acr-config --enable-containerd
        # The enablement binary paths are overridable (ACR_MIRROR_SETUP_SCRIPT /
        # ACR_CONFIG_BIN), so the stubs live in a temp dir instead of mutating /opt.
        setup_streaming() {
            TEST_ACR_DIR="$(mktemp -d)"
            ACR_MIRROR_SETUP_SCRIPT="${TEST_ACR_DIR}/setup.sh"
            ACR_CONFIG_BIN="${TEST_ACR_DIR}/acr-config"
        }
        cleanup_streaming() {
            rm -rf "${TEST_ACR_DIR}"
        }
        BeforeEach 'setup_streaming'
        AfterEach 'cleanup_streaming'

        waitForContainerdReady() {
            return 0
        }
        systemctl() {
            echo "systemctl $@"
        }
        retrycmd_if_failure() {
            echo "retrycmd_if_failure $@"
            return "${RETRYCMD_RC:-0}"
        }

        install_setup_sh() {
            printf '#!/bin/sh\necho "setup.sh $@"\n' > "${ACR_MIRROR_SETUP_SCRIPT}"
            chmod +x "${ACR_MIRROR_SETUP_SCRIPT}"
        }
        install_acr_config() {
            printf '#!/bin/sh\necho "acr-config $@"\n' > "${ACR_CONFIG_BIN}"
            chmod +x "${ACR_CONFIG_BIN}"
        }

        It 'uses setup.sh aks when acr-mirror 1.0.0+ is installed'
            install_setup_sh
            When run ensureArtifactStreaming
            The output should include "setup.sh aks"
            The output should not include "Older acr-mirror package"
            The status should be success
        End

        It 'falls back to acr-config enablement when setup.sh is absent (older package)'
            install_acr_config
            When run ensureArtifactStreaming
            The output should include "Older acr-mirror package is detected"
            The output should include "acr-config --enable-containerd azurecr.io"
            The status should be success
        End

        It 'fails fast when enabling the streaming services fails'
            RETRYCMD_RC=1
            install_setup_sh
            When run ensureArtifactStreaming
            The output should include "retrycmd_if_failure"
            The output should not include "setup.sh aks"
            The status should equal "$ERR_ARTIFACT_STREAMING_INSTALL"
        End
    End
    Describe 'cleanUpGridNodeCudaPrebake'
        # Stub the actual removal so tests assert the keep-vs-teardown DECISION without touching the
        # real filesystem. OS defaults to Ubuntu (the only path this function acts on).
        OS="$UBUNTU_OS_NAME"
        # shellcheck disable=SC2329 # invoked dynamically by cleanUpGridNodeCudaPrebake.
        cleanUpPrebakedGPUDriver() { echo "STUB_TEARDOWN_CALLED"; }

        It 'tears down a cuda prebake before installing a GRID driver (A10/GRID outage path)'
            marker="$(mktemp)"
            printf 'driver_kind=cuda\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            When call cleanUpGridNodeCudaPrebake
            The output should include "action=teardown"
            The output should include "marker_kind=cuda"
            The output should include "node_kind=grid"
            The output should include "STUB_TEARDOWN_CALLED"
            rm -f "$marker"
        End

        It 'tears down for a grid-v20 node whose driver-type maps to grid'
            marker="$(mktemp)"
            printf 'driver_kind=cuda\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="grid-v20"
            When call cleanUpGridNodeCudaPrebake
            The output should include "action=teardown"
            The output should include "STUB_TEARDOWN_CALLED"
            rm -f "$marker"
        End

        It 'treats a legacy marker without driver_kind as a cuda prebake and tears down on a GRID node'
            marker="$(mktemp)"
            printf 'kernel=5.15.0-1114-azure\n' > "$marker"   # no driver_kind= line
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            When call cleanUpGridNodeCudaPrebake
            The output should include "action=teardown"
            The output should include "marker_kind=none"
            The output should include "STUB_TEARDOWN_CALLED"
            rm -f "$marker"
        End

        It 'is a no-op on a CUDA node (leaves the cuda prebake for the version-match/library-bump path)'
            marker="$(mktemp)"
            printf 'driver_kind=cuda\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="cuda-lts"
            When call cleanUpGridNodeCudaPrebake
            The output should not include "STUB_TEARDOWN_CALLED"
            The status should be success
            rm -f "$marker"
        End

        It 'is a no-op when the prebake is already grid (matches the grid node)'
            marker="$(mktemp)"
            printf 'driver_kind=grid\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            When call cleanUpGridNodeCudaPrebake
            The output should not include "STUB_TEARDOWN_CALLED"
            The status should be success
            rm -f "$marker"
        End

        It 'is a no-op when no prebake marker exists'
            GPU_DKMS_MARKER_FILE="$(mktemp)"; rm -f "${GPU_DKMS_MARKER_FILE}"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            When call cleanUpGridNodeCudaPrebake
            The output should not include "STUB_TEARDOWN_CALLED"
            The status should be success
        End

        It 'is a no-op on a non-Ubuntu OS even when a mismatched marker is present'
            marker="$(mktemp)"
            printf 'driver_kind=cuda\n' > "$marker"
            GPU_DKMS_MARKER_FILE="$marker"
            OS="MARINER"   # override the Ubuntu default set at the Describe level
            NVIDIA_GPU_DRIVER_TYPE="grid"
            When call cleanUpGridNodeCudaPrebake
            The output should not include "STUB_TEARDOWN_CALLED"
            The status should be success
            OS="$UBUNTU_OS_NAME"   # restore for any subsequent examples
            rm -f "$marker"
        End
    End
    Describe 'configureManagedGPUExperience'
        # Mock the helper functions
        logs_to_events() {
            echo "logs_to_events $1 $2"
            eval "$2"
        }

        installNvidiaManagedExpPkgFromCache() {
            echo "installNvidiaManagedExpPkgFromCache called"
            return 0
        }

        startNvidiaManagedExpServices() {
            echo "startNvidiaManagedExpServices called"
            return 0
        }

        systemctlDisableAndStop() {
            echo "systemctlDisableAndStop $1"
            return 0
        }

        addKubeletNodeLabel() {
            echo "addKubeletNodeLabel $1"
            if [[ -z "$KUBELET_NODE_LABELS" ]]; then
                KUBELET_NODE_LABELS="$1"
            else
                KUBELET_NODE_LABELS="$KUBELET_NODE_LABELS,$1"
            fi
        }

        mkdir() {
            echo "mkdir $@"
        }

        touch() {
            echo "touch $@"
        }

        rm() {
            echo "rm $@"
        }

        BeforeEach 'KUBELET_NODE_LABELS=""'

        It 'should not enable managed GPU experience if not GPU node'
            GPU_NODE="false"

            When call configureManagedGPUExperience

            The output should not include "installNvidiaManagedExpPkgFromCache called"
            The output should not include "startNvidiaManagedExpServices called"
            The output should not include "addKubeletNodeLabel kubernetes.azure.com/dcgm-exporter=enabled"
            The output should not include "touch /opt/azure/containers/managed-gpu-experience.enabled"
            The output should not include "rm -f /opt/azure/containers/managed-gpu-experience.enabled"
        End

        It 'should not enable managed GPU experience when skip_nvidia_driver_install is true'
            GPU_NODE="true"
            skip_nvidia_driver_install="true"
            ENABLE_MANAGED_GPU_EXPERIENCE="true"

            When call configureManagedGPUExperience

            The output should not include "installNvidiaManagedExpPkgFromCache called"
            The output should not include "startNvidiaManagedExpServices called"
            The output should not include "addKubeletNodeLabel kubernetes.azure.com/dcgm-exporter=enabled"
            The output should not include "touch /opt/azure/containers/managed-gpu-experience.enabled"
            The output should not include "rm -f /opt/azure/containers/managed-gpu-experience.enabled"
        End

        It 'should not enable managed GPU experience when ENABLE_MANAGED_GPU_EXPERIENCE is unspecified'
            GPU_NODE="true"
            skip_nvidia_driver_install="false"
            ENABLE_MANAGED_GPU_EXPERIENCE=""

            When call configureManagedGPUExperience

            The output should not include "installNvidiaManagedExpPkgFromCache called"
            The output should not include "startNvidiaManagedExpServices called"
            The output should not include "addKubeletNodeLabel kubernetes.azure.com/dcgm-exporter=enabled"
            The output should include "rm -f /opt/azure/containers/managed-gpu-experience.enabled"
        End

        It 'should enable managed GPU experience when ENABLE_MANAGED_GPU_EXPERIENCE is true'
            GPU_NODE="true"
            skip_nvidia_driver_install="false"
            ENABLE_MANAGED_GPU_EXPERIENCE="true"

            When call configureManagedGPUExperience

            The output should include "installNvidiaManagedExpPkgFromCache called"
            The output should include "startNvidiaManagedExpServices called"
            The output should include "addKubeletNodeLabel kubernetes.azure.com/dcgm-exporter=enabled"
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/dcgm-exporter=enabled'
            The output should include "mkdir -p /opt/azure/containers"
            The output should include "touch /opt/azure/containers/managed-gpu-experience.enabled"
        End

        It 'should disable managed GPU experience when ENABLE_MANAGED_GPU_EXPERIENCE is false'
            GPU_NODE="true"
            skip_nvidia_driver_install="false"
            ENABLE_MANAGED_GPU_EXPERIENCE="false"

            When call configureManagedGPUExperience

            The output should include "systemctlDisableAndStop nvidia-device-plugin"
            The output should include "systemctlDisableAndStop nvidia-dcgm"
            The output should include "systemctlDisableAndStop nvidia-dcgm-exporter"
            The output should not include "addKubeletNodeLabel kubernetes.azure.com/dcgm-exporter=enabled"
            The output should include "rm -f /opt/azure/containers/managed-gpu-experience.enabled"
        End
    End
    Describe 'startNvidiaManagedExpServices'
        logs_to_events() {
            echo "logs_to_events $1"
            eval "$2"
        }
        systemctlEnableAndStart() {
            echo "systemctlEnableAndStart $@"
        }
        systemctlEnableAndStartNoBlock() {
            echo "systemctlEnableAndStartNoBlock $@"
        }
        mkdir() {
            echo "mkdir $@"
        }
        tee() {
            cat > /dev/null
            echo "tee $@"
        }
        systemctl() {
            echo "systemctl $@"
        }

        BeforeEach 'MIG_NODE="false"; ENABLE_MANAGED_GPU_EXPERIENCE="true"; ENABLE_MANAGED_GPU_EXPERIENCE_DRA="false"'

        It 'starts the device-plugin blocking but dcgm and dcgm-exporter off the critical path'
            When call startNvidiaManagedExpServices

            # device-plugin gates GPU scheduling, so it must stay blocking.
            The output should include "systemctlEnableAndStart nvidia-device-plugin 30"
            # dcgm/dcgm-exporter are telemetry only and must not block provisioning.
            The output should include "systemctlEnableAndStartNoBlock nvidia-dcgm 30"
            The output should include "systemctlEnableAndStartNoBlock nvidia-dcgm-exporter 30"
            The output should not include "systemctlEnableAndStart nvidia-dcgm 30"
            The output should not include "systemctlEnableAndStart nvidia-dcgm-exporter 30"
        End

        It 'does not fail when dcgm telemetry services cannot be enqueued'
            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
                return 1
            }

            When call startNvidiaManagedExpServices

            The status should be success
            The output should include "warning: nvidia-dcgm could not be enqueued"
            The output should include "warning: nvidia-dcgm-exporter could not be enqueued"
        End

        It 'starts the DRA driver blocking but dcgm and dcgm-exporter off the critical path in DRA mode'
            ENABLE_MANAGED_GPU_EXPERIENCE="false"
            ENABLE_MANAGED_GPU_EXPERIENCE_DRA="true"

            When call startNvidiaManagedExpServices

            The output should include "systemctlEnableAndStart dra-driver-nvidia-gpu 30"
            The output should include "systemctlEnableAndStartNoBlock nvidia-dcgm 30"
            The output should include "systemctlEnableAndStartNoBlock nvidia-dcgm-exporter 30"
            The output should not include "systemctlEnableAndStart nvidia-device-plugin 30"
            The output should not include "systemctlEnableAndStart nvidia-dcgm 30"
            The output should not include "systemctlEnableAndStart nvidia-dcgm-exporter 30"
        End
    End
    Describe 'nvidia-cdi-refresh handling'
        setup_cdi_dropin() {
            CDI_TEST_DIR=$(mktemp -d)
            NVIDIA_CDI_REFRESH_DROP_IN="${CDI_TEST_DIR}/nvidia-cdi-refresh.service.d/10-aks-tolerate-generate-failure.conf"
        }
        cleanup_cdi_dropin() {
            rm -rf "${CDI_TEST_DIR}"
        }
        BeforeEach 'setup_cdi_dropin'
        AfterEach 'cleanup_cdi_dropin'

        Describe 'configureNvidiaCDIRefresh'
            It 'treats the known nvidia-ctk failures as success and reloads systemd'
                systemctl() { echo "systemctl $*"; return 0; }

                When call configureNvidiaCDIRefresh

                The status should be success
                The output should include "systemctl daemon-reload"
                # 1 = MIG mode enabled with no MIG instances, 2 = nvidia-ctk panic before the
                # driver userspace is ready, 127 = nvidia-smi missing on pre-reorder aks-gpu images.
                The contents of file "${NVIDIA_CDI_REFRESH_DROP_IN}" should include "[Service]"
                The contents of file "${NVIDIA_CDI_REFRESH_DROP_IN}" should include "SuccessExitStatus=1 2 127"
            End

            It 'keeps the units restartable so the .path trigger can still refresh the spec'
                systemctl() { return 0; }

                When call configureNvidiaCDIRefresh

                The status should be success
                # Overriding Restart= or the start limit would suppress the toolkit's own retry and
                # its path-triggered refresh, which is what eventually generates a valid spec once
                # the driver is fully installed.
                The contents of file "${NVIDIA_CDI_REFRESH_DROP_IN}" should not include "Restart="
                The contents of file "${NVIDIA_CDI_REFRESH_DROP_IN}" should not include "StartLimit"
                The contents of file "${NVIDIA_CDI_REFRESH_DROP_IN}" should not include "ExecCondition"
            End

            It 'still installs the drop-in when systemd cannot be reloaded'
                # Drop-ins are read when the unit is first loaded, which happens after the toolkit
                # post-install runs its own daemon-reload, so this reload is only belt and braces.
                systemctl() { return 1; }

                When call configureNvidiaCDIRefresh

                The status should be success
                The contents of file "${NVIDIA_CDI_REFRESH_DROP_IN}" should include "SuccessExitStatus=1 2 127"
            End

            It 'fails when the drop-in directory cannot be created'
                systemctl() { return 0; }
                mkdir() { return 1; }

                When call configureNvidiaCDIRefresh

                The status should be failure
            End
        End
    End
    Describe 'selectGPUDriverImage'
        setup_gpu_image() {
            OS="UBUNTU"
            GPU_DRIVER_TYPE="grid"
            GPU_DRIVER_VERSION="570.237"
            GPU_IMAGE_SHA="20260817204535"
            GPU_DV="$GPU_DRIVER_VERSION"
            NVIDIA_GPU_DRIVER_TYPE="$GPU_DRIVER_TYPE"
            NVIDIA_DRIVER_IMAGE_SHA="$GPU_IMAGE_SHA"
            NVIDIA_DRIVER_IMAGE_TAG="${GPU_DRIVER_VERSION}-${GPU_IMAGE_SHA}"
            NVIDIA_DRIVER_IMAGE="mcr.microsoft.com/aks/aks-gpu-grid"
            NVIDIA_DRIVER_IMAGE_PULL_REF="$NVIDIA_DRIVER_IMAGE"
            unset MCR_REPOSITORY_BASE
            COMPONENTS_FILEPATH="parts/common/components.json"
            expected_grid_v20_tag=$(jq -r '.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*") | .gpuVersion.latestVersion' "$COMPONENTS_FILEPATH")
        }
        BeforeEach 'setup_gpu_image'

        Describe 'Ubuntu GRID selection'
            Parameters
                "18.04"
                "20.04"
                "22.04"
                "24.04"
                "26.04"
            End

            It 'selects the complete GRID v20 pin on every Ubuntu release'
                OS_VERSION="$1"
                When call selectGPUDriverImage

                The status should be success
                The output should include "Ubuntu GRID driver image: mcr.microsoft.com/aks/aks-gpu-grid-v20:$expected_grid_v20_tag"
                The variable GPU_DRIVER_TYPE should equal "grid-v20"
                The variable NVIDIA_GPU_DRIVER_TYPE should equal "grid-v20"
                The variable GPU_DRIVER_VERSION should equal "${expected_grid_v20_tag%-*}"
                The variable GPU_DV should equal "${expected_grid_v20_tag%-*}"
                The variable GPU_IMAGE_SHA should equal "${expected_grid_v20_tag##*-}"
                The variable NVIDIA_DRIVER_IMAGE_SHA should equal "${expected_grid_v20_tag##*-}"
                The variable NVIDIA_DRIVER_IMAGE_TAG should equal "$expected_grid_v20_tag"
                The variable NVIDIA_DRIVER_IMAGE should equal "mcr.microsoft.com/aks/aks-gpu-grid-v20"
                The variable NVIDIA_DRIVER_IMAGE_PULL_REF should equal "mcr.microsoft.com/aks/aks-gpu-grid-v20"
            End
        End

        It 'preserves the cloud-specific pull registry and canonical image name'
            MCR_REPOSITORY_BASE="mcr.example/"
            When call selectGPUDriverImage

            The status should be success
            The output should include "Ubuntu GRID driver image:"
            The variable NVIDIA_DRIVER_IMAGE should equal "mcr.microsoft.com/aks/aks-gpu-grid-v20"
            The variable NVIDIA_DRIVER_IMAGE_PULL_REF should equal "mcr.example/aks/aks-gpu-grid-v20"
            The variable NVIDIA_DRIVER_IMAGE_TAG should equal "$expected_grid_v20_tag"
        End

        Describe 'unchanged selections'
            Parameters
                "AZURELINUX" "" "grid"
                "MARINER" "" "grid"
                "AZURELINUX" "AZURECONTAINERLINUX" "grid"
                "AZURECONTAINERLINUX" "AZURECONTAINERLINUX" "grid"
                "UBUNTU" "" "cuda-lts"
                "UBUNTU" "" "cuda"
                "UBUNTU" "" "grid-v20"
                "UBUNTU" "" ""
            End

            It 'does not change image inputs or read components.json'
                OS="$1"
                OS_VARIANT="$2"
                NVIDIA_GPU_DRIVER_TYPE="$3"
                COMPONENTS_FILEPATH="not-needed.json"
                When call selectGPUDriverImage

                The status should be success
                The output should be blank
                The variable NVIDIA_GPU_DRIVER_TYPE should equal "$3"
                The variable GPU_DV should equal "570.237"
                The variable NVIDIA_DRIVER_IMAGE_TAG should equal "570.237-20260817204535"
                The variable NVIDIA_DRIVER_IMAGE should equal "mcr.microsoft.com/aks/aks-gpu-grid"
                The variable NVIDIA_DRIVER_IMAGE_PULL_REF should equal "mcr.microsoft.com/aks/aks-gpu-grid"
            End
        End

        Describe 'unreadable components'
            Parameters
                "missing-grid-v20-components.json"
                "/dev/null"
            End

            It 'fails without changing the image'
                COMPONENTS_FILEPATH="$1"
                When call selectGPUDriverImage

                The status should be failure
                The stderr should include "Ubuntu GRID requires a valid aks-gpu-grid-v20 pin"
                The variable NVIDIA_GPU_DRIVER_TYPE should equal "grid"
                The variable NVIDIA_DRIVER_IMAGE_TAG should equal "570.237-20260817204535"
            End
        End

        Describe 'invalid GRID v20 pins'
            setup_invalid_pin() {
                COMPONENTS_FILEPATH=$(mktemp)
                jq "$1" parts/common/components.json > "$COMPONENTS_FILEPATH"
            }
            cleanup_invalid_pin() { rm -f "$COMPONENTS_FILEPATH"; }
            AfterEach 'cleanup_invalid_pin'

            Parameters
                '"invalid components document"'
                'del(.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*"))'
                '(.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*") | .gpuVersion.latestVersion) = "595.91.07"'
                '(.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*") | .gpuVersion.latestVersion) = "570.237-20260817204535"'
                '(.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*") | .gpuVersion.latestVersion) = null'
                '.GPUContainerImages += [.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*")]'
            End

            It 'rejects invalid documents and missing, malformed, wrong-branch, or duplicate pins'
                setup_invalid_pin "$1"
                When call selectGPUDriverImage

                The status should be failure
                The stderr should include "Ubuntu GRID requires a valid aks-gpu-grid-v20 pin"
                The variable NVIDIA_GPU_DRIVER_TYPE should equal "grid"
                The variable NVIDIA_DRIVER_IMAGE_TAG should equal "570.237-20260817204535"
            End
        End
    End
    Describe 'configGPUDrivers'
        # Assert the per-step CSE timing event names emitted via logs_to_events,
        # without running the real (hardware/daemon) driver steps. logs_to_events
        # is mocked to print only the event name so the wrapped commands never run.
        logs_to_events() {
            echo "logs_to_events $1"
        }
        waitForContainerdReady() { return 0; }
        retrycmd_if_failure() { return 0; }
        ctr() { return 0; }
        mkdir() { return 0; }
        enableNvidiaPersistenceMode() { return 0; }
        createNvidiaSymlinkToAllDeviceNodes() { return 0; }
        systemctlEnableAndStart() { return 0; }
        systemctl() { return 0; }
        configureNvidiaCDIRefresh() { return 0; }

        UBUNTU_OS_NAME="UBUNTU"
        NVIDIA_DRIVER_IMAGE="mcr.example/nvidia/driver"
        NVIDIA_DRIVER_IMAGE_TAG="000.00"
        NVIDIA_GPU_DRIVER_TYPE="cuda"
        OS_VARIANT=""
        ERR_GPU_DRIVERS_START_FAIL=88

        It 'selects GRID v20 before checking the cache, pulling, installing, and removing the image'
            OS="UBUNTU"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            COMPONENTS_FILEPATH="parts/common/components.json"
            MCR_REPOSITORY_BASE="mcr.example/"
            expected_grid_v20_tag=$(jq -r '.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*") | .gpuVersion.latestVersion' "$COMPONENTS_FILEPATH")
            logs_to_events() { shift; eval "$@"; }
            ctr() { echo "ctr $*" >&2; }
            pullGPUDriverImage() { echo "pull $NVIDIA_DRIVER_IMAGE_PULL_REF:$NVIDIA_DRIVER_IMAGE_TAG"; }
            installGPUDriverImage() { echo "install $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG"; }

            When call configGPUDrivers

            The status should be success
            The stderr should include "images ls -q name==mcr.microsoft.com/aks/aks-gpu-grid-v20:$expected_grid_v20_tag"
            The output should include "pull mcr.example/aks/aks-gpu-grid-v20:$expected_grid_v20_tag"
            The output should include "install mcr.microsoft.com/aks/aks-gpu-grid-v20:$expected_grid_v20_tag"
            The stderr should include "images rm mcr.microsoft.com/aks/aks-gpu-grid-v20:$expected_grid_v20_tag"
        End

        It 'does not pull or install an old GRID driver when the v20 pin is missing'
            OS="UBUNTU"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            COMPONENTS_FILEPATH="missing-grid-v20-components.json"
            When run configGPUDrivers

            The status should equal 88
            The stderr should include "Ubuntu GRID requires a valid aks-gpu-grid-v20 pin"
            The output should be blank
        End

        It 'uses the cached v20 image without pulling the legacy GRID image'
            OS="UBUNTU"
            NVIDIA_GPU_DRIVER_TYPE="grid"
            COMPONENTS_FILEPATH="parts/common/components.json"
            expected_grid_v20_tag=$(jq -r '.GPUContainerImages[] | select(.downloadURL == "mcr.microsoft.com/aks/aks-gpu-grid-v20:*") | .gpuVersion.latestVersion' "$COMPONENTS_FILEPATH")
            logs_to_events() { shift; eval "$@"; }
            ctr() {
                if [ "$3 $4 $5" = "images ls -q" ]; then
                    echo "mcr.microsoft.com/aks/aks-gpu-grid-v20:$expected_grid_v20_tag"
                fi
            }
            pullGPUDriverImage() { echo "PULL_RAN"; return 1; }
            installGPUDriverImage() { echo "install $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG"; }

            When call configGPUDrivers

            The status should be success
            The output should include "install mcr.microsoft.com/aks/aks-gpu-grid-v20:$expected_grid_v20_tag"
            The output should not include "PULL_RAN"
        End

        It 'times the image pull and install steps on Ubuntu'
            OS="UBUNTU"
            isMarinerOrAzureLinux() { return 1; }
            isAzureLinuxOSGuard() { return 1; }
            isACL() { return 1; }

            When call configGPUDrivers

            The status should be success
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.pullGPUDriverImage"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.installGPUDriverImage"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.waitForNvidiaModprobe"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.waitForNvidiaSmi"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.configureNvidiaCDIRefresh"
        End

        It 'installs the CDI refresh drop-in before the driver install runs on Ubuntu'
            OS="UBUNTU"
            isMarinerOrAzureLinux() { return 1; }
            isAzureLinuxOSGuard() { return 1; }
            isACL() { return 1; }
            # eval so the string-form wrapped commands (e.g. "retrycmd_if_failure ... nvidia-smi")
            # resolve to their mocks instead of being treated as one command name.
            logs_to_events() { shift; eval "$@"; }
            NVIDIA_DRIVER_IMAGE_PULL_REF="mcr.example/nvidia/driver"
            configureNvidiaCDIRefresh() { echo "CONFIGURE_RAN"; return 0; }
            installGPUDriverImage() { echo "INSTALL_RAN"; return 0; }

            When call configGPUDrivers

            The status should be success
            # The toolkit starts nvidia-cdi-refresh from inside the install container, so the
            # drop-in must already be on disk by then.
            The line 1 of output should equal "CONFIGURE_RAN"
            The output should include "INSTALL_RAN"
        End

        It 'exits without installing the driver when the CDI drop-in cannot be installed'
            OS="UBUNTU"
            isMarinerOrAzureLinux() { return 1; }
            isAzureLinuxOSGuard() { return 1; }
            isACL() { return 1; }
            logs_to_events() { shift; eval "$@"; }
            configureNvidiaCDIRefresh() { return 1; }
            installGPUDriverImage() { echo "INSTALL_RAN"; return 0; }

            When run configGPUDrivers

            The status should equal 88
            The output should not include "INSTALL_RAN"
        End

        It 'exits at the cache-miss pull step and skips install when the pull fails on Ubuntu'
            OS="UBUNTU"
            isMarinerOrAzureLinux() { return 1; }
            isAzureLinuxOSGuard() { return 1; }
            isACL() { return 1; }
            # Run the wrapped command so pullGPUDriverImage's failure propagates through the guard;
            # the default logs_to_events mock only echoes the event name and never runs the command.
            logs_to_events() { shift; "$@"; }
            # ctr images ls returns empty -> cache miss -> pull path taken. The pull fails; install
            # would otherwise succeed, so a missing guard would let INSTALL_RAN leak into the output.
            pullGPUDriverImage() { return 1; }
            installGPUDriverImage() { echo "INSTALL_RAN"; return 0; }

            When run configGPUDrivers

            The status should equal 88
            The output should not include "INSTALL_RAN"
        End

        It 'times the driver download and toolkit install on Mariner/AzureLinux'
            OS="AZURELINUX"
            isMarinerOrAzureLinux() { return 0; }
            isAzureLinuxOSGuard() { return 1; }
            isACL() { return 1; }

            When call configGPUDrivers

            The status should be success
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.downloadGPUDrivers"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.installNvidiaContainerToolkit"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.waitForNvidiaModprobe"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.waitForNvidiaSmi"
        End

        It 'times the sysext pulls on Azure Container Linux (ACL)'
            OS="AZURELINUX"
            OS_VARIANT="acl"
            isMarinerOrAzureLinux() { return 0; }
            isAzureLinuxOSGuard() { return 0; }
            isACL() { return 0; }

            When call configGPUDrivers

            The status should be success
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.installNvidiaContainerToolkitSysext"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.installGPUDriverSysext"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.waitForNvidiaModprobe"
            The output should include "logs_to_events AKS.CSE.configGPUDrivers.waitForNvidiaSmi"
        End
    End
    Describe 'managedGPUPackageList on Ubuntu'
        Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

        BeforeEach 'setup'
        setup() {
            ENABLE_MANAGED_GPU_EXPERIENCE=""
            ENABLE_MANAGED_GPU_EXPERIENCE_DRA=""
        }

        It 'returns base managed GPU packages by default'
            When call managedGPUPackageList

            The status should be success
            The output should equal 'datacenter-gpu-manager-4-core datacenter-gpu-manager-4-proprietary dcgm-exporter'
            The output should not include 'nvidia-device-plugin'
            The output should not include 'dra-driver-nvidia-gpu'
        End

        It 'includes nvidia-device-plugin when managed GPU experience is enabled'
            ENABLE_MANAGED_GPU_EXPERIENCE="true"

            When call managedGPUPackageList

            The status should be success
            The output should include 'datacenter-gpu-manager-4-core'
            The output should include 'datacenter-gpu-manager-4-proprietary'
            The output should include 'dcgm-exporter'
            The output should include 'nvidia-device-plugin'
            The output should not include 'dra-driver-nvidia-gpu'
        End

        It 'includes dra-driver-nvidia-gpu when DRA mode is enabled'
            ENABLE_MANAGED_GPU_EXPERIENCE_DRA="true"

            When call managedGPUPackageList

            The status should be success
            The output should include 'datacenter-gpu-manager-4-core'
            The output should include 'datacenter-gpu-manager-4-proprietary'
            The output should include 'dcgm-exporter'
            The output should include 'dra-driver-nvidia-gpu'
            The output should not include 'nvidia-device-plugin'
        End
    End
    Describe "setupAmdAma"

        uname() {
            echo "6.6.139.1-1.azl3"
        }

        dnf_install() {
            return 0
        }

        dnf_install_amd_ama_core_package() {
            return 0
        }

        systemctl() {
            return 0
        }

        sh() {
            return 0
        }

        BeforeEach 'OS=AZURELINUX'

        It "selects the newest matching AMD AMA driver package"
            dnf() {
                cat <<EOF
amd-ama-driver-0:1.4.0_20260424092403-1_6.6.139.1.1.azl3.x86_64.rpm
amd-ama-driver-0:1.4.1_20260424092403-1_6.6.139.1.1.azl3.x86_64.rpm
amd-ama-driver-0:1.5.0_20260424092403-1_6.6.139.1.1.azl3.x86_64.rpm
EOF
            }

            When call setupAmdAma

            The status should be success
            The variable AMD_AMA_DRIVER_PACKAGE should equal \
                "amd-ama-driver-0:1.5.0_20260424092403-1_6.6.139.1.1.azl3.x86_64.rpm"
            The variable AMD_AMA_DRIVER_VERSION should equal "1.5.0"
        End
    End
End
