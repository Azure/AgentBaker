#!/bin/bash
# shellcheck disable=SC2016,SC2034,SC2329

Describe 'NVMe read-ahead VHD integration'
  load_helpers() {
    . ./parts/linux/cloud-init/artifacts/cse_helpers.sh >/dev/null
    . ./vhdbuilder/packer/packer_source.sh
    cpAndMode() { printf '%s\n' "$*"; }
  }
  BeforeAll 'load_helpers'

  Describe 'included OS families'
    Parameters
      UBUNTU 20.04 ''
      UBUNTU 22.04 ''
      UBUNTU 24.04 ''
      UBUNTU 26.04 ''
      MARINER 2.0 ''
      MARINERKATA 2.0 ''
      AZURELINUX 3.0 ''
      AZURELINUXKATA 3.0 ''
      AZURELINUX 4.0 ''
      AZURELINUX 3.0 OSGUARD
      AZURELINUX 3.0 AZURECONTAINERLINUX
      AZURECONTAINERLINUX 3.0 ''
    End
    It "installs all assets on $1 $2 variant=$3 regardless of build hardware"
      OS="$1" OS_VERSION="$2" OS_VARIANT="$3"
      When call copyNVMeReadAheadFiles
      The status should be success
      The lines of output should equal 4
      The output should include '/opt/azure/containers/azure-nvme-readahead.sh 755'
      The output should include '/etc/udev/rules.d/99-azure-nvme-readahead.rules 644'
      The output should include '/etc/systemd/system/azure-nvme-readahead.service 644'
      The output should include '/etc/systemd/system/azure-nvme-readahead.timer 644'
    End
  End

  It 'does not install assets on Flatcar'
    OS=FLATCAR OS_VERSION=4230.2.2 OS_VARIANT=''
    When call copyNVMeReadAheadFiles
    The status should be success
    The output should be blank
  End

  Describe 'Packer uploads'
    Parameters
      base
      cvm
      arm64-gen2
      arm64-gb
      mariner
      mariner-arm64
      mariner-cvm
      acl
      acl-arm64
    End
    It "uploads each exact asset once in $1"
      filter='[.provisioners[] | select(.type == "file")] as $files |
        ["sh", "rules", "service", "timer"] | all(. as $ext |
        [$files[] | select(.source == ("parts/linux/cloud-init/artifacts/azure-nvme-readahead." + $ext)
          and .destination == ("/home/packer/azure-nvme-readahead." + $ext))] | length == 1)'
      When call jq -e "$filter" "vhdbuilder/packer/vhd-image-builder-$1.json"
      The status should be success
      The output should equal true
    End
  End

  Describe 'OSGuard uploads'
    Parameters
      sh /opt/azure/containers/azure-nvme-readahead.sh 755
      rules /etc/udev/rules.d/99-azure-nvme-readahead.rules 644
      service /etc/systemd/system/azure-nvme-readahead.service 644
      timer /etc/systemd/system/azure-nvme-readahead.timer 644
    End
    It "installs the $1 asset into the writable filesystem"
      When call grep -A2 -F "source: /AgentBaker/parts/linux/cloud-init/artifacts/azure-nvme-readahead.$1" \
        vhdbuilder/packer/imagecustomizer/azlosguard/azlosguard.yml
      The output should include "destination: $2"
      The output should include "permissions: $3"
    End
  End

  It 'calls the copier from the VHD file installation path'
    When call sed -n '/^copyPackerFiles()/,/^}/p' vhdbuilder/packer/packer_source.sh
    The output should include '  copyNVMeReadAheadFiles'
  End

  It 'enables both units without starting them on the build VM'
    When call grep -F 'systemctl enable azure-nvme-readahead.service' vhdbuilder/packer/install-dependencies.sh
    The output should equal '  systemctl enable azure-nvme-readahead.service azure-nvme-readahead.timer || exit 1'
  End

  It 'limits udev writes to the known-bad value and affected model'
    When call cat parts/linux/cloud-init/artifacts/azure-nvme-readahead.rules
    The output should include 'ACTION=="add|change", SUBSYSTEM=="block", ENV{DEVTYPE}=="disk"'
    The output should include 'ENV{ID_MODEL}=="MSFT NVMe Accelerator v1.0"'
    The output should include 'ATTR{queue/read_ahead_kb}=="65536", ATTR{queue/read_ahead_kb}="256"'
    The output should include 'ACTION=="add|change|remove"'
    The output should include 'RUN+="/opt/azure/containers/azure-nvme-readahead.sh"'
  End

  It 'runs before kubelet at boot and exits within a bounded time'
    When call cat parts/linux/cloud-init/artifacts/azure-nvme-readahead.service
    The output should include 'Before=kubelet.service'
    The output should include 'TimeoutStartSec=10s'
    The output should include 'WantedBy=multi-user.target'
    The output should not include 'RemainAfterExit'
  End

  It 'rechecks after eventless rescans'
    When call cat parts/linux/cloud-init/artifacts/azure-nvme-readahead.timer
    The output should include 'OnBootSec=30s'
    The output should include 'OnUnitInactiveSec=30s'
    The output should include 'AccuracySec=1s'
  End
End

Describe 'NVMe read-ahead post-boot content test'
  setup_validation() {
    TEST_ROOT="${SHELLSPEC_TMPBASE}/nvme-vhd-validation"
    mkdir -p "$TEST_ROOT/etc/udev/rules.d" "$TEST_ROOT/etc/systemd/system" \
      "$TEST_ROOT/opt/azure/containers" "$TEST_ROOT/sys/block/nvme0n1/device" \
      "$TEST_ROOT/sys/block/nvme0n1/queue"
    cp parts/linux/cloud-init/artifacts/azure-nvme-readahead.rules "$TEST_ROOT/etc/udev/rules.d/99-azure-nvme-readahead.rules"
    cp parts/linux/cloud-init/artifacts/azure-nvme-readahead.sh "$TEST_ROOT/opt/azure/containers/"
    chmod 755 "$TEST_ROOT/opt/azure/containers/azure-nvme-readahead.sh"
    cp parts/linux/cloud-init/artifacts/azure-nvme-readahead.service \
      parts/linux/cloud-init/artifacts/azure-nvme-readahead.timer "$TEST_ROOT/etc/systemd/system/"
    chmod 644 "$TEST_ROOT/etc/udev/rules.d/99-azure-nvme-readahead.rules" "$TEST_ROOT/etc/systemd/system/"*
    printf '%s\n' 'MSFT NVMe Accelerator v1.0' > "$TEST_ROOT/sys/block/nvme0n1/device/model"
    printf '%s\n' 256 > "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb"
    sed -n '/^testNVMeReadAhead()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh |
      sed -e "s|/opt/azure/|$TEST_ROOT/opt/azure/|g" \
          -e "s|/etc/|$TEST_ROOT/etc/|g" \
          -e "s|/sys/block/|$TEST_ROOT/sys/block/|g" > "$TEST_ROOT/validation.sh"
    . "$TEST_ROOT/validation.sh"
    OS_SKU=Ubuntu
    SYSTEMCTL_STATUS=0
  }
  cleanup_validation() { rm -rf "$TEST_ROOT"; }
  BeforeEach 'setup_validation'
  AfterEach 'cleanup_validation'
  err() { printf '%s:Error: %s\n' "$1" "$2" >&2; }
  systemctl() { return "$SYSTEMCTL_STATUS"; }
  stat() {
    if [ "$(uname -s)" = Darwin ]; then
      command stat -f '%Lp' "$3"
    else
      command stat "$@"
    fi
  }

  It 'accepts a corrected boot without writing to sysfs'
    When call testNVMeReadAhead
    The status should be success
    The output should be blank
    The contents of file "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb" should equal 256
  End

  It 'reports the real post-boot failure instead of correcting it in the test'
    printf '%s\n' 65536 > "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb"
    When call testNVMeReadAhead
    The error should include 'testNVMeReadAhead:Error:'
    The error should include 'still has excessive or unreadable read-ahead'
    The contents of file "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb" should equal 65536
  End

  It 'reports disabled or inactive units'
    SYSTEMCTL_STATUS=1
    When call testNVMeReadAhead
    The error should include 'must be installed with mode 0644 and enabled'
    The error should include 'reconciliation timer is not active'
  End

  It 'reports a missing rule'
    rm "$TEST_ROOT/etc/udev/rules.d/99-azure-nvme-readahead.rules"
    When call testNVMeReadAhead
    The error should include 'Missing or incorrect NVMe read-ahead udev rule'
  End

  It 'ignores a different NVMe model'
    printf '%s\n' 'Microsoft NVMe Direct Disk' > "$TEST_ROOT/sys/block/nvme0n1/device/model"
    printf '%s\n' 65536 > "$TEST_ROOT/sys/block/nvme0n1/queue/read_ahead_kb"
    When call testNVMeReadAhead
    The status should be success
    The output should be blank
  End

  It 'skips Flatcar'
    OS_SKU=Flatcar SYSTEMCTL_STATUS=1
    When call testNVMeReadAhead
    The status should be success
    The output should be blank
  End
End
