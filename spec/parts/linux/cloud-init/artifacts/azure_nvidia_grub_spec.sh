#!/bin/bash

Describe 'NVIDIA GRUB kernel selection'
  select_kernels() {
    export reverse_sorted_list="$1"
    export GRUB_NVIDIA_KERNEL_VERSION="$2"
    export boot_device_id="test-device"
    # Supply the kernel inventory instead of sourcing the host's GRUB generator.
    sed '\|^\. /etc/grub.d/10_linux |d' ./parts/linux/cloud-init/artifacts/10_azure_nvidia | sh
  }

  It 'selects the unified GB kernel for NVIDIA CPUs and LTS for other CPUs'
    When call select_kernels '/boot/vmlinuz-7.0.0-1020-azure /boot/vmlinuz-6.8.0-1073-azure' '7.0.0-1020-azure'
    The status should be success
    The output should include 'if [ x$cpu_manufacturer = xNVIDIA ]; then'
    The output should include 'set default="gnulinux-7.0.0-1020-azure-advanced-test-device"'
    The output should include 'set default="gnulinux-6.8.0-1073-azure-advanced-test-device"'
    The output should include 'set nvidia_args="iommu.passthrough=1 irqchip.gicv3_nolpi=y arm_smmu_v3.disable_msipolling=1"'
    The stderr should include 'Default NVIDIA image: /boot/vmlinuz-7.0.0-1020-azure'
    The stderr should include 'Default non-NVIDIA image: /boot/vmlinuz-6.8.0-1073-azure'
  End

  It 'preserves legacy NVIDIA kernel selection without a GB override'
    When call select_kernels '/boot/vmlinuz-6.14.0-1003-azure-nvidia /boot/vmlinuz-6.8.0-1073-azure' ''
    The status should be success
    The output should include 'set default="gnulinux-6.14.0-1003-azure-nvidia-advanced-test-device"'
    The output should include 'set default="gnulinux-6.8.0-1073-azure-advanced-test-device"'
    The stderr should include 'Default NVIDIA image: /boot/vmlinuz-6.14.0-1003-azure-nvidia'
  End

  It 'does not classify unified kernels as NVIDIA on non-GB images'
    When call select_kernels '/boot/vmlinuz-7.0.0-1020-azure /boot/vmlinuz-6.8.0-1073-azure' ''
    The status should be success
    The output should eq ''
    The stderr should eq 'Only one image type (NVIDIA or non-NVIDIA) found'
  End
End
