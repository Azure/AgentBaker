#!/bin/bash

Describe 'addMarinerNvidiaRepo'
    setup() {
        TEST_DIR=$(mktemp -d)
        eval "$(sed -n '/^addMarinerNvidiaRepo()/,/^}/p' './vhdbuilder/scripts/linux/mariner/tool_installs_mariner.sh' | sed "s|/etc/yum.repos.d|${TEST_DIR}|g")"
    }

    cleanup() {
        rm -rf "$TEST_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'Azure Linux 3'
        Parameters
            x86_64
            aarch64
        End

        It "leaves architecture substitution to the package manager on $1"
            OS_VERSION=3.0
            basearch="$1"

            When call addMarinerNvidiaRepo

            The status should be success
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include "name=Azure Linux Official Nvidia 3.0 \$basearch"
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include "baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/nvidia/\$basearch/"
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include 'gpgcheck=1'
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include 'repo_gpgcheck=1'
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include 'sslverify=1'
            The path "${TEST_DIR}/mariner-nvidia.repo" should not be exist
        End
    End

    It 'preserves the Mariner 2 repository'
        OS_VERSION=2.0

        When call addMarinerNvidiaRepo

        The status should be success
        The contents of file "${TEST_DIR}/mariner-nvidia.repo" should include 'baseurl=https://packages.microsoft.com/cbl-mariner/2.0/prod/nvidia/x86_64'
        The path "${TEST_DIR}/azurelinux-nvidia.repo" should not be exist
    End
End
Describe 'Azure Linux 4 unattended upgrade units'
    setup() {
        TEST_DIR=$(mktemp -d)
        SYSTEMCTL_LOG="${TEST_DIR}/systemctl.log"
        export AZL4_SYSTEMD_UNIT_DIR="${TEST_DIR}/units"
        export AZL4_CHECK_RESTART_BIN_DIR="${TEST_DIR}/bin"
        ERR_SYSTEMCTL_START_FAIL=4
        ERR_SYSTEMCTL_MASK_FAIL=2
        for fn in installAzureLinux4DnfAutomaticInstallUnits installAzureLinux4CheckRestart disableDNFAutomatic enableCheckRestart; do
            eval "$(sed -n "/^${fn}()/,/^}/p" './vhdbuilder/scripts/linux/mariner/tool_installs_mariner.sh')"
        done
        systemctl() { echo "systemctl $*" >> "$SYSTEMCTL_LOG"; }
        systemctlEnableAndStart() { echo "systemctlEnableAndStart $*" >> "$SYSTEMCTL_LOG"; }
    }

    cleanup() {
        rm -rf "$TEST_DIR"
        unset AZL4_SYSTEMD_UNIT_DIR AZL4_CHECK_RESTART_BIN_DIR
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'installs dnf-automatic-install units that run dnf5 automatic with updates applied'
        When call installAzureLinux4DnfAutomaticInstallUnits

        The status should be success
        The contents of file "${AZL4_SYSTEMD_UNIT_DIR}/dnf-automatic-install.service" should include 'ExecStart=/usr/bin/dnf5 automatic --timer --installupdates'
        The contents of file "${AZL4_SYSTEMD_UNIT_DIR}/dnf-automatic-install.timer" should include 'OnCalendar=*-*-* 6:00'
        The contents of file "${AZL4_SYSTEMD_UNIT_DIR}/dnf-automatic-install.timer" should include 'RandomizedDelaySec=60m'
        The contents of file "${AZL4_SYSTEMD_UNIT_DIR}/dnf-automatic-install.timer" should include 'WantedBy=timers.target'
        The contents of file "$SYSTEMCTL_LOG" should equal 'systemctl daemon-reload'
    End

    It 'installs a check-restart service, timer and script that use dnf5 needs-restarting'
        When call installAzureLinux4CheckRestart

        The status should be success
        The contents of file "${AZL4_CHECK_RESTART_BIN_DIR}/check-restart.sh" should include "OUTPUT=\$(dnf5 needs-restarting 2>&1)"
        The contents of file "${AZL4_CHECK_RESTART_BIN_DIR}/check-restart.sh" should include 'touch /var/run/reboot-required'
        The path "${AZL4_CHECK_RESTART_BIN_DIR}/check-restart.sh" should be executable
        The contents of file "${AZL4_SYSTEMD_UNIT_DIR}/check-restart.service" should include "ExecStart=/bin/bash ${AZL4_CHECK_RESTART_BIN_DIR}/check-restart.sh"
        The contents of file "${AZL4_SYSTEMD_UNIT_DIR}/check-restart.timer" should include 'OnCalendar=*-*-* 08:00:00'
    End

    It 'flags a pending reboot only when dnf5 reports one'
        installAzureLinux4CheckRestart
        script="${TEST_DIR}/check-restart-test.sh"
        sed "s|/var/run/reboot-required|${TEST_DIR}/reboot-required|" "${AZL4_CHECK_RESTART_BIN_DIR}/check-restart.sh" > "$script"
        mkdir -p "${TEST_DIR}/stub"
        printf '#!/bin/bash\necho "Updated packages: kernel"\necho "Reboot is required to fully utilize these updates."\nexit 1\n' > "${TEST_DIR}/stub/dnf5"
        chmod +x "${TEST_DIR}/stub/dnf5"
        run_with_stub() { PATH="${TEST_DIR}/stub:$PATH" bash "$script"; }

        When call run_with_stub

        The status should be success
        The output should include 'Reboot is required'
        The path "${TEST_DIR}/reboot-required" should be file
    End

    It 'does not flag a reboot when dnf5 says none is needed'
        installAzureLinux4CheckRestart
        script="${TEST_DIR}/check-restart-test.sh"
        sed "s|/var/run/reboot-required|${TEST_DIR}/reboot-required|" "${AZL4_CHECK_RESTART_BIN_DIR}/check-restart.sh" > "$script"
        mkdir -p "${TEST_DIR}/stub"
        printf '#!/bin/bash\necho "No core libraries or services have been updated since boot-up."\necho "Reboot should not be necessary."\n' > "${TEST_DIR}/stub/dnf5"
        chmod +x "${TEST_DIR}/stub/dnf5"
        run_with_stub() { PATH="${TEST_DIR}/stub:$PATH" bash "$script"; }

        When call run_with_stub

        The status should be success
        The output should include 'Reboot should not be necessary'
        The path "${TEST_DIR}/reboot-required" should not be exist
    End

    It 'masks the install units and dnf5 timers on Azure Linux 4 without touching notify-only units'
        OS_VERSION=4.0

        When call disableDNFAutomatic

        The status should be success
        The path "${AZL4_SYSTEMD_UNIT_DIR}/dnf-automatic-install.timer" should be file
        The contents of file "$SYSTEMCTL_LOG" should include 'systemctl disable --now dnf5-automatic.timer dnf-automatic.timer'
        The contents of file "$SYSTEMCTL_LOG" should include 'systemctl mask dnf5-automatic.service dnf5-automatic.timer dnf-automatic.service dnf-automatic.timer'
        The contents of file "$SYSTEMCTL_LOG" should include 'systemctl mask dnf-automatic-install.service dnf-automatic-install.timer'
        The contents of file "$SYSTEMCTL_LOG" should not include 'notifyonly'
    End

    It 'keeps the Azure Linux 3 behaviour'
        OS_VERSION=3.0

        When call disableDNFAutomatic

        The status should be success
        The contents of file "$SYSTEMCTL_LOG" should include 'systemctl mask dnf-automatic-notifyonly.timer'
        The contents of file "$SYSTEMCTL_LOG" should include 'systemctl mask dnf-automatic-install.timer'
        The path "${AZL4_SYSTEMD_UNIT_DIR}" should not be exist
    End

    It 'installs and enables check-restart on Azure Linux 4'
        OS_VERSION=4.0

        When call enableCheckRestart

        The status should be success
        The path "${AZL4_SYSTEMD_UNIT_DIR}/check-restart.timer" should be file
        The contents of file "$SYSTEMCTL_LOG" should include 'systemctlEnableAndStart check-restart.timer 30'
    End

    It 'only enables the packaged check-restart on Azure Linux 3'
        OS_VERSION=3.0

        When call enableCheckRestart

        The status should be success
        The contents of file "$SYSTEMCTL_LOG" should equal 'systemctlEnableAndStart check-restart.timer 30'
        The path "${AZL4_CHECK_RESTART_BIN_DIR}" should not be exist
    End
End
