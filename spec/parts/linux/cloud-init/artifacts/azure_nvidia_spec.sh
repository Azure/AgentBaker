#!/bin/bash

Describe '10_azure_nvidia'
    SCRIPT='./parts/linux/cloud-init/artifacts/10_azure_nvidia'

    Mock rpm
        for arg do
            kernel_path=$arg
        done
        case "${kernel_path##*/}" in
            vmlinuz-1.0.*) printf 'kernel' ;;
            vmlinuz-2.0.*) printf 'kernel-hwe' ;;
            *) exit 1 ;;
        esac
    End

    setup() {
        TEST_DIR=$(mktemp -d)
        BOOT_DIR="${TEST_DIR}/boot"
        GRUB_LINUX_SCRIPT="${TEST_DIR}/10_linux"
        mkdir -p "$BOOT_DIR"
        for kernel in 1.0.9-test 1.0.10-test 2.0.9-test 2.0.10-test 9.0.0-unowned; do
            touch "${BOOT_DIR}/vmlinuz-${kernel}"
        done
        cat > "$GRUB_LINUX_SCRIPT" <<'EOF'
boot_device_id=test-device
grub_probe=module_probe
module_probe() {
    [ "$1" = --target=device ] && [ "$2" = /usr/lib/grub/arm64-efi ] || return 1
    printf '%s\n' /dev/module-device
}
is_path_readable_by_grub() {
    [ "$1" = /usr/lib/grub/arm64-efi ]
}
prepare_grub_to_access_device() {
    [ "$1" = /dev/module-device ] || return 1
    printf '%s\n' 'search --no-floppy --fs-uuid --set=root module-filesystem'
}
make_system_path_relative_to_its_root() {
    [ "$1" = /usr/lib/grub ] || return 1
    printf '%s\n' /lib/grub
}
EOF
        export BOOT_DIR GRUB_LINUX_SCRIPT
    }

    cleanup() {
        rm -rf "$TEST_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'selects the newest RPM-owned kernel from each Azure Linux track'
        When run script "$SCRIPT"
        The status should be success
        The output should include 'set default="gnulinux-2.0.10-test-advanced-test-device"'
        The output should include 'set default="gnulinux-1.0.10-test-advanced-test-device"'
        The output should not include '9.0.0-unowned'
        The stderr should include 'Default NVIDIA image: 2.0.10-test, non-NVIDIA image: 1.0.10-test'
    End

    It 'preserves Ubuntu kernel selection by filename'
        rm -f "${BOOT_DIR}"/vmlinuz-*
        cat > "$GRUB_LINUX_SCRIPT" <<'EOF'
boot_device_id=test-device
reverse_sorted_list='/boot/vmlinuz-3.0.10-azure-nvidia /boot/vmlinuz-3.0.10-azure'
EOF
        When run script "$SCRIPT"
        The status should be success
        The output should include 'set default="gnulinux-3.0.10-azure-nvidia-advanced-test-device"'
        The output should include 'set default="gnulinux-3.0.10-azure-advanced-test-device"'
        The output should include 'insmod smbios'
        The output should not include 'azure_nvidia_saved_prefix'
        The stderr should include 'Default NVIDIA image: 3.0.10-azure-nvidia, non-NVIDIA image: 3.0.10-azure'
    End

    It 'loads package-owned modules from their filesystem and restores GRUB state'
        When run script "$SCRIPT"
        The status should be success
        The output should include 'set azure_nvidia_saved_root="$root"
set azure_nvidia_saved_prefix="$prefix"
search --no-floppy --fs-uuid --set=root module-filesystem
set prefix="($root)/lib/grub"
insmod smbios
set prefix="$azure_nvidia_saved_prefix"
set root="$azure_nvidia_saved_root"
unset azure_nvidia_saved_prefix
unset azure_nvidia_saved_root'
        The output should not include '/boot/grub2/arm64-efi'
        The stderr should include 'Default NVIDIA image:'
    End

    It 'fails generation when GRUB cannot read the module filesystem'
        printf '%s\n' 'is_path_readable_by_grub() { return 1; }' >> "$GRUB_LINUX_SCRIPT"
        When run script "$SCRIPT"
        The status should be failure
        The output should eq ''
        The stderr should include 'GRUB cannot read module directory: /usr/lib/grub/arm64-efi'
    End

    It 'fails generation when probing the module device fails'
        printf '%s\n' 'module_probe() { return 1; }' >> "$GRUB_LINUX_SCRIPT"
        When run script "$SCRIPT"
        The status should be failure
        The output should eq ''
        The stderr should include 'Default NVIDIA image:'
    End

    It 'emits no override when either kernel track is missing'
        rm -f "${BOOT_DIR}"/vmlinuz-2.0.*
        When run script "$SCRIPT"
        The status should be success
        The output should eq ''
        The stderr should include 'Only one image type (NVIDIA or non-NVIDIA) found'
    End

    It 'emits no override when 10_linux cannot derive a boot device ID'
        printf '%s\n' 'boot_device_id=' > "$GRUB_LINUX_SCRIPT"
        When run script "$SCRIPT"
        The status should be success
        The output should eq ''
        The stderr should include "Could not determine boot_device_id from $GRUB_LINUX_SCRIPT"
    End
End