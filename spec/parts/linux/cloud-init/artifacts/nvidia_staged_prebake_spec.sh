#!/bin/bash

Describe 'whole NVIDIA prebake staging'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"

    setup() {
        GPU_PREBAKE_ROOT="${SHELLSPEC_TMPBASE}/nvidia-stage"
        mkdir -p "$GPU_PREBAKE_ROOT/lib/modules"
        cache="${GPU_PREBAKE_ROOT}/opt/azure/aks-gpu/staged"
        GPU_DKMS_MARKER_FILE="${GPU_PREBAKE_ROOT}/opt/azure/aks-gpu/dkms-marker"
        GPU_NODE=true
        skip_nvidia_driver_install=false
        NVIDIA_GPU_DRIVER_TYPE=cuda-lts
        GPU_DV=580.159.04
        CONFIG_GPU_DRIVER_IF_NEEDED=false
        OS=UBUNTU UBUNTU_OS_NAME=UBUNTU ERR_GPU_DRIVERS_START_FAIL=84
        prepareStagedGPUDriver
        mkdir -p "${GPU_PREBAKE_ROOT}"/{var/lib/nvidia,var/lib/dkms/nvidia/580.159.04,usr/src/nvidia-580.159.04,usr/bin/lib64/lib64,etc/ld.so.conf.d,etc/modprobe.d,lib/modules/test-kernel/updates/dkms,boot,usr/share/glvnd/egl_vendor.d,usr/lib/systemd/system}
        ln -s /usr/src/nvidia-580.159.04 "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia/580.159.04/source"
        printf 'source\n' > "${GPU_PREBAKE_ROOT}/usr/src/nvidia-580.159.04/dkms.conf"
        printf 'driver_version=580.159.04\nkernel=test-kernel\narch=x86_64\ndriver_kind=cuda\n' > "$GPU_DKMS_MARKER_FILE"
        local path
        for path in usr/bin/nvidia-modprobe usr/bin/lib64/lib64/libcuda.so.580 usr/share/glvnd/egl_vendor.d/10_nvidia.json \
            usr/lib/systemd/system/nvidia-powerd.service etc/modprobe.d/blacklist-nouveau.conf etc/ld.so.conf.d/nvidia.conf \
            lib/modules/test-kernel/updates/dkms/nvidia.ko.zst boot/initrd.img-test-kernel; do
            printf 'payload\n' > "${GPU_PREBAKE_ROOT}/${path}"
        done
        chmod 4755 "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe"
        ln -s libcuda.so.580 "${GPU_PREBAKE_ROOT}/usr/bin/lib64/lib64/libcuda.so.1"
        ln -s missing-target "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-uninstall"
        printf 'unrelated\n' > "${GPU_PREBAKE_ROOT}/usr/bin/unrelated"
        printf 'unrelated\n' > "${GPU_PREBAKE_ROOT}/lib/modules/test-kernel/updates/dkms/other.ko"
        cat > "${GPU_PREBAKE_ROOT}/var/lib/nvidia/log" <<'EOF'
580.159.04
NVIDIA Accelerated Graphics Driver
1: /usr/bin/nvidia-modprobe
123
0: /usr/bin/nvidia-uninstall
missing-target
1: /usr/share/glvnd/egl_vendor.d/10_nvidia.json
234
1: /usr/lib/systemd/system/nvidia-powerd.service
345
1: /usr/lib/x86_64-linux-gnu/libcuda.so.580
456
1: /usr/src/nvidia-580.159.04/dkms.conf
567
EOF
    }
    cleanup() { command rm -rf "${GPU_PREBAKE_ROOT}"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    # Only system commands are mocked. Ownership inventory, chmod, symlinks and renames are real.
    lsmod() { :; }
    depmod() { printf 'depmod %s\n' "$*" >> "${GPU_PREBAKE_ROOT}/refresh"; [ "${FAIL_REFRESH:-}" != depmod ]; }
    ldconfig() { echo ldconfig >> "${GPU_PREBAKE_ROOT}/refresh"; [ "${FAIL_REFRESH:-}" != ldconfig ]; }
    systemctl() { echo systemctl >> "${GPU_PREBAKE_ROOT}/refresh"; [ "${FAIL_REFRESH:-}" != systemctl ]; }
    update-initramfs() { echo initramfs >> "${GPU_PREBAKE_ROOT}/refresh"; [ "${FAIL_REFRESH:-}" != initramfs ]; }
    lsinitramfs() { printf '%s\n' "${INITRD_CONTENT:-kernel/other.ko}"; [ "${FAIL_REFRESH:-}" != lsinitramfs ]; }
    logs_to_events() { shift; ${@}; }
    isARM64() { echo 0; }
    uname() { if [ "$1" = -m ]; then echo x86_64; else echo "${LIVE_KERNEL:-test-kernel}"; fi; }
    dkms() { printf '%s\n' "${DKMS_STATUS:-nvidia/580.159.04, test-kernel, x86_64: installed}"; }
    modinfo() { echo 580.159.04; }
    configGPUDrivers() {
        test -L "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia/580.159.04/source" || return
        echo INSTALL
        [ "${FAIL_INSTALL:-}" != true ]
    }
    validateGPUDrivers() { test -L "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia/580.159.04/source" && echo VALIDATE; }
    systemctlEnableAndStart() { :; }
    logGPUDriverPrebakeReadiness() { :; }

    It 'stages installer-owned files, configuration, DKMS and modules but preserves unrelated files and sources'
        When call stageGPUDriver
        The status should be success
        The output should equal 'AKS_GPU_PREBAKE event=staged'
        The path "${cache}/staged" should be exist
        The path "${cache}/files/usr/bin/nvidia-modprobe" should be file
        The path "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe" should not be exist
        The path "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia" should not be exist
        The path "${GPU_PREBAKE_ROOT}/usr/share/glvnd/egl_vendor.d/10_nvidia.json" should not be exist
        The path "${GPU_PREBAKE_ROOT}/usr/lib/systemd/system/nvidia-powerd.service" should not be exist
        The path "${GPU_PREBAKE_ROOT}/usr/src/nvidia-580.159.04/dkms.conf" should be file
        The contents of file "${GPU_PREBAKE_ROOT}/usr/bin/unrelated" should equal unrelated
        The contents of file "${GPU_PREBAKE_ROOT}/lib/modules/test-kernel/updates/dkms/other.ko" should equal unrelated
    End

    It 'makes cached setuid binaries root-only, then preserves mode and symlinks on restore'
        round_trip() {
            stageGPUDriver >/dev/null || return
            stat -c '%u:%g:%a' "$cache"
            stat -c '%a' "${cache}/files/usr/bin/nvidia-modprobe"
            restoreStagedGPUDriver || return
            stat -c '%a' "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe"
            readlink "${GPU_PREBAKE_ROOT}/usr/bin/lib64/lib64/libcuda.so.1"
            readlink "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-uninstall"
            readlink "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia/580.159.04/source"
        }
        When call round_trip
        The status should be success
        The output should include '0:0:700'
        The output should include '4755'
        The output should include 'libcuda.so.580'
        The output should include 'missing-target'
        The output should include '/usr/src/nvidia-580.159.04'
        The path "$cache" should not be exist
    End

    Context 'live node eligibility'
    Parameters
        true false cuda-lts
        false false cuda-lts
        true true cuda-lts
        true false grid
        true false grid-v20
    End
    It "uses live node decisions ($1/$2/$3), not a PIS basePrep decision"
        stageGPUDriver >/dev/null
        GPU_NODE="$1" skip_nvidia_driver_install="$2" NVIDIA_GPU_DRIVER_TYPE="$3"
        When call restoreStagedGPUDriver
        The status should be success
        The output should match pattern 'AKS_GPU_PREBAKE event=*'
        The path "$cache" should not be exist
        if [ "$1/$2/$3" = true/false/cuda-lts ]; then
            The path "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe" should be file
        else
            The path "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe" should not be exist
            The path "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia" should not be exist
        fi
    End

    End
    Context 'dispatch'
    Parameters
        true INSTALL
        false INSTALL
    End
    It "restores DKMS before dispatch and initializes build-only even when validation was requested ($1)"
        stageGPUDriver >/dev/null
        CONFIG_GPU_DRIVER_IF_NEEDED="$1"
        When run ensureGPUDrivers
        The status should be success
        The output should include 'AKS_GPU_PREBAKE event=restored'
        The output should include "$2"
    End

    End
    Context 'repair'
    Parameters
        kernel
        version
        registration
    End
    It "repairs a $1 mismatch through the normal installer even on validation-only nodes"
        stageGPUDriver >/dev/null
        restoreStagedGPUDriver >/dev/null
        removeStagedGPUDriver installed >/dev/null
        case "$1" in
            kernel) LIVE_KERNEL=new-kernel ;;
            version) GPU_DV=590.1.2 ;;
            registration) DKMS_STATUS=unregistered ;;
        esac
        When run ensureGPUDrivers
        The status should be success
        The output should include event=repair_required
        The output should include INSTALL
        The output should not include VALIDATE
    End

    End
    Context 'stage refresh failures'
    Parameters
        depmod
        ldconfig
        systemctl
        initramfs
        lsinitramfs
    End
    It "keeps a failed stage visible and retryable after $1 fails"
        FAIL_REFRESH="$1"
        When call stageGPUDriver
        The status should be failure
        The path "${cache}/manifest" should be file
        The path "${cache}/staged" should not be exist
    End

    End
    Context 'restore refresh failures'
    Parameters
        depmod
        ldconfig
        systemctl
    End
    It "retries restoration after $1 fails without merging installations"
        retry_restore() {
            stageGPUDriver >/dev/null || return
            FAIL_REFRESH="$1"
            restoreStagedGPUDriver && return 1
            test -f "${cache}/restoring" || return
            FAIL_REFRESH=
            restoreStagedGPUDriver
        }
        When call retry_restore "$1"
        The status should be success
        The output should equal 'AKS_GPU_PREBAKE event=restored'
    End

    End
    It 'refuses to certify an initramfs that still contains NVIDIA modules'
        INITRD_CONTENT='usr/lib/modules/test-kernel/updates/dkms/nvidia.ko.zst'
        When call stageGPUDriver
        The status should be failure
        The stderr should include 'NVIDIA payload remains'
        The path "${cache}/staged" should not be exist
    End

    It 'fails staging if NVIDIA modules exist outside the supported owned locations'
        mkdir -p "${GPU_PREBAKE_ROOT}/lib/modules/test-kernel/extra"
        touch "${GPU_PREBAKE_ROOT}/lib/modules/test-kernel/extra/nvidia.ko"
        When call stageGPUDriver
        The status should be failure
        The stderr should include 'Unstaged NVIDIA modules remain'
        The path "${cache}/staged" should not be exist
    End

    It 'does not dispose of a partially staged transaction on an opted-out node'
        FAIL_REFRESH=depmod
        stageGPUDriver >/dev/null || :
        When call removeStagedGPUDriver
        The status should be failure
        The stderr should include 'transaction is incomplete'
        The path "${cache}/files/usr/bin/nvidia-modprobe" should be file
    End

    It 'rejects existing producer state instead of merging it into a new cache'
        When call prepareStagedGPUDriver
        The status should be failure
        The stderr should include 'existing /opt/azure/aks-gpu/staged'
    End

    Context 'pre-existing NVIDIA payload ownership'
        Parameters
            module
            library
        End
        It "refuses a pre-existing $1 rather than claiming it as the new producer's artifact"
            command rm -rf "$GPU_PREBAKE_ROOT"
            mkdir -p "${GPU_PREBAKE_ROOT}/lib/modules/independent/updates/dkms"
            if [ "$1" = module ]; then
                touch "${GPU_PREBAKE_ROOT}/lib/modules/independent/updates/dkms/nvidia.ko"
            else
                ldconfig() { echo 'libnvidia-ml.so.1 => /usr/lib/x86_64-linux-gnu/libnvidia-ml.so.1'; }
            fi
            When call prepareStagedGPUDriver
            The status should be failure
            The stderr should include 'existing NVIDIA modules or linker entries'
            The path "$cache" should not be exist
        End
    End

    It 'preserves file ownership across staging and restoration'
        ownership() {
            chown 123:456 "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe" || return
            stageGPUDriver >/dev/null || return
            restoreStagedGPUDriver >/dev/null || return
            stat -c '%u:%g' "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe"
        }
        When call ownership
        The status should be success
        The output should equal '123:456'
    End

    It 'uses the normal legacy dispatch when no staged cache exists'
        command rm -rf "$cache"
        When run ensureGPUDrivers
        The status should be success
        The output should equal VALIDATE
    End

    It 'refuses to overwrite an independent installation before moving anything'
        stageGPUDriver >/dev/null
        printf 'independent\n' > "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe"
        When call restoreStagedGPUDriver
        The status should be failure
        The stderr should include collision
        The contents of file "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe" should equal independent
        The path "${cache}/files/var/lib/dkms/nvidia" should be directory
    End

    It 'rejects a successful mv that did not move anything'
        mv() { case "$*" in *'-n'*) return 0 ;; *) command mv "$@" ;; esac; }
        When call stageGPUDriver
        The status should be failure
        The path "${cache}/staged" should not be exist
    End

    It 'resumes a partial rename transaction'
        retry_stage() {
            local moves=0
            mv() {
                case "$*" in *'-n'*) moves=$((moves + 1)); [ "$moves" -ne 3 ] || return 1 ;; esac
                command mv "$@"
            }
            stageGPUDriver && return 1
            unset -f mv
            stageGPUDriver
        }
        When call retry_stage
        The status should be success
        The output should equal 'AKS_GPU_PREBAKE event=staged'
    End

    It 'refuses cross-filesystem staging before moving files'
        stat() {
            case "$*" in *'/usr/bin/nvidia-modprobe') echo other-device ;; *) command stat "$@" ;; esac
        }
        When call stageGPUDriver
        The status should be failure
        The path "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia" should be directory
        The path "${GPU_PREBAKE_ROOT}/usr/bin/nvidia-modprobe" should be file
    End

    It 'rejects unowned replacements from the installer inventory'
        printf '100: /usr/bin/unrelated\n123 100755 0 0\n' >> "${GPU_PREBAKE_ROOT}/var/lib/nvidia/log"
        When call stageGPUDriver
        The status should be failure
        The contents of file "${GPU_PREBAKE_ROOT}/usr/bin/unrelated" should equal unrelated
        The path "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia" should be directory
    End

    It 'reports inactive-cache removal failure without claiming an active registration'
        stageGPUDriver >/dev/null
        rm() { return 1; }
        When call cleanUpPrebakedGPUDriver
        The status should be success
        The stderr should include 'event=inactive_cache_cleanup status=incomplete'
        The path "${GPU_PREBAKE_ROOT}/var/lib/dkms/nvidia" should not be exist
        unset -f rm
    End

    It 'keeps ordinary runtime initialization mandatory after an installer failure and CSE retry'
        retry_install() {
            stageGPUDriver >/dev/null || return
            FAIL_INSTALL=true
            local rc=0
            (ensureGPUDrivers) || rc=$?
            [ "$rc" = 84 ] && [ -d "${cache}-needs-install" ] || return 1
            FAIL_INSTALL=false CONFIG_GPU_DRIVER_IF_NEEDED=false
            ensureGPUDrivers
        }
        When run retry_install
        The status should be success
        The output should include event=initialization_required
        The output should include INSTALL
        The output should not include VALIDATE
        The path "${cache}-needs-install" should not be exist
    End

    Context 'damaged but inactive cache'
        Parameters
            /var/lib/dkms/nvidia
            /lib/modules/test-kernel/updates/dkms/nvidia.ko.zst
            /usr/bin/nvidia-modprobe
        End
        It "uses the ordinary installer rather than failing when cached $1 is missing"
            stageGPUDriver >/dev/null
            command rm -rf "${cache}/files$1"
            configGPUDrivers() { echo INSTALL; }
            When run ensureGPUDrivers
            The status should be success
            The output should include 'event=cache_incomplete action=normal_install'
            The output should include INSTALL
            The output should not include event=restored
            The path "${cache}-needs-install" should not be exist
        End
    End

    Context 'interrupted final disposal'
        Parameters
            inactive
            installed
        End
        It "retries $1 disposal after the manifest and sentinel have already been removed"
            retry_disposal() {
                stageGPUDriver >/dev/null || return
                if [ "$1" = installed ]; then restoreStagedGPUDriver >/dev/null || return; fi
                rm() {
                    command rm -f "${cache}-discard/manifest" "${cache}-discard/staged" "${cache}-discard/restoring"
                    return 1
                }
                removeStagedGPUDriver "$1" && return 1
                [ -d "${cache}-discard" ] || return
                unset -f rm
                removeStagedGPUDriver "$1"
            }
            When call retry_disposal "$1"
            The status should be success
            The path "${cache}-discard" should not be exist
            The path "$cache" should not be exist
        End
    End
End
