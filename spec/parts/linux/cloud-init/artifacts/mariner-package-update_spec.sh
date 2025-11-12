#!/bin/bash

Describe 'mariner-package-update.sh'
    setup() {
        Include "./parts/linux/cloud-init/artifacts/mariner/mariner-package-update.sh"
        TEST_DIR="/tmp/live-patching-test"
        mkdir -p ${TEST_DIR}
        OS_RELEASE_FILE="${TEST_DIR}/os-release"
        SECURITY_PATCH_REPO_DIR="${TEST_DIR}"
        KUBECONFIG="${TEST_DIR}/kubeconfig"
        touch "${KUBECONFIG}"
        KUBECTL="kubectl"
    }
    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'Mariner 2.0'
        setup_os_release() {
            cat <<EOF > "${OS_RELEASE_FILE}"
NAME="Common Base Linux Mariner"
VERSION="2.0.20250701"
ID=mariner
VERSION_ID="2.0"
PRETTY_NAME="CBL-Mariner/Linux"
ANSI_COLOR="1;34"
HOME_URL="https://aka.ms/cbl-mariner"
BUG_REPORT_URL="https://aka.ms/cbl-mariner"
SUPPORT_URL="https://aka.ms/cbl-mariner"
EOF
        }
        BeforeEach 'setup_os_release'

        Mock dnf
            echo "dnf mock called with args: $@"
        End

        repo_config=$(cat <<'EOF'
[mariner-official-base]
name=CBL-Mariner Official Base $releasever $basearch
baseurl=https://packages.microsoft.com/cbl-mariner/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        custom_cloud_repo_config=$(cat <<'EOF'
[mariner-official-base]
name=CBL-Mariner Official Base $releasever $basearch
baseurl=https://repodepot.azure.microsoft.fakecustomcloud/mariner/packages.microsoft.com/cbl-mariner/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        ni_repo_config=$(cat <<'EOF'
# original_baseurl=https://packages.microsoft.com
[mariner-official-base]
name=CBL-Mariner Official Base $releasever $basearch
baseurl=http://10.0.0.1/cbl-mariner/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        # repo service IP changes
        ni_repo_config_2=$(cat <<'EOF'
# original_baseurl=https://packages.microsoft.com
[mariner-official-base]
name=CBL-Mariner Official Base $releasever $basearch
baseurl=http://10.0.0.2/cbl-mariner/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        # original_baseurl changes
        ni_repo_config_3=$(cat <<'EOF'
# original_baseurl=https://repodepot.azure.microsoft.fakecustomcloud/mariner/packages.microsoft.com
[mariner-official-base]
name=CBL-Mariner Official Base $releasever $basearch
baseurl=http://10.0.0.1/cbl-mariner/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY file:///etc/pki/rpm-gpg/MICROSOFT-METADATA-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        It 'should update successfully for regular cluster'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                fi
            End

            echo "${repo_config}" > "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is not set, use PMC repo'
            # repo config doesn't change
            The output should not include "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo is updated"
            The output should include "dnf mock called with args: update --exclude mshv-linuxloader --exclude kernel-mshv --repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo" should eq "${repo_config}"
        End

        It 'should update successfully for ni cluster'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                elif [[ "$@" == *"live-patching-repo-service"* ]]; then
                    echo "10.0.0.1"
                fi
            End

            echo "${repo_config}" > "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is: 10.0.0.1, use it to replace PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo is updated"
            The output should include "dnf mock called with args: update --exclude mshv-linuxloader --exclude kernel-mshv --repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo" should eq "${ni_repo_config}"
        End

        It 'should update successfully for ni cluster when repo service changes'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                elif [[ "$@" == *"live-patching-repo-service"* ]]; then
                    echo "10.0.0.2"
                fi
            End

            echo "${ni_repo_config}" > "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is: 10.0.0.2, use it to replace PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo is updated"
            The output should include "dnf mock called with args: update --exclude mshv-linuxloader --exclude kernel-mshv --repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo" should eq "${ni_repo_config_2}"
        End

        It 'should update successfully when cluster changed from ni to regular'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                fi
            End

            echo "${ni_repo_config}" > "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is not set, use PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo is updated"
            The output should include "dnf mock called with args: update --exclude mshv-linuxloader --exclude kernel-mshv --repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo" should eq "${repo_config}"
        End

        It 'should update successfully for custom cloud cluster'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                elif [[ "$@" == *"live-patching-repo-service"* ]]; then
                    echo "10.0.0.1"
                fi
            End

            echo "${custom_cloud_repo_config}" > "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is: 10.0.0.1, use it to replace PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo is updated"
            The output should include "dnf mock called with args: update --exclude mshv-linuxloader --exclude kernel-mshv --repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/mariner-official-base.repo" should eq "${ni_repo_config_3}"
        End

        It 'should do nothing if golden timestamp is not set'
            Mock kubectl
                echo ""
            End
            When run main
            The status should be success
            The output should include 'golden timestamp is not set, skip live patching'
        End

        It 'should do nothing if golden timestamp equals current timestamp'
            Mock kubectl
                echo "20250820T000000Z"
            End
            When run main
            The status should be success
            The output should include 'golden timestamp is: 20250820T000000Z'
            The output should include 'current timestamp is: 20250820T000000Z'
            The output should include 'golden and current timestamp is the same, nothing to patch'
        End
    End

    Describe 'AzureLinux 3.0'
        setup_os_release() {
            cat <<EOF > "${OS_RELEASE_FILE}"
NAME="Microsoft Azure Linux"
VERSION="3.0.20250702"
ID=azurelinux
VERSION_ID="3.0"
PRETTY_NAME="Microsoft Azure Linux 3.0"
ANSI_COLOR="1;34"
HOME_URL="https://aka.ms/azurelinux"
BUG_REPORT_URL="https://aka.ms/azurelinux"
SUPPORT_URL="https://aka.ms/azurelinux"
EOF
        }
        BeforeEach 'setup_os_release'

        Mock tdnf
            echo "tdnf mock called with args: $@"
        End

        repo_config=$(cat <<'EOF'
[azurelinux-official-base]
name=Azure Linux Official Base $releasever $basearch
baseurl=https://packages.microsoft.com/azurelinux/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        custom_cloud_repo_config=$(cat <<'EOF'
[azurelinux-official-base]
name=Azure Linux Official Base $releasever $basearch
baseurl=https://repodepot.azure.microsoft.fakecustomcloud/mariner/packages.microsoft.com/azurelinux/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        ni_repo_config=$(cat <<'EOF'
# original_baseurl=https://packages.microsoft.com
[azurelinux-official-base]
name=Azure Linux Official Base $releasever $basearch
baseurl=http://10.0.0.1/azurelinux/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        # repo service IP changes
        ni_repo_config_2=$(cat <<'EOF'
# original_baseurl=https://packages.microsoft.com
[azurelinux-official-base]
name=Azure Linux Official Base $releasever $basearch
baseurl=http://10.0.0.2/azurelinux/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        # original_baseurl changes
        ni_repo_config_3=$(cat <<'EOF'
# original_baseurl=https://repodepot.azure.microsoft.fakecustomcloud/mariner/packages.microsoft.com
[azurelinux-official-base]
name=Azure Linux Official Base $releasever $basearch
baseurl=http://10.0.0.1/azurelinux/$releasever/prod/base/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
gpgcheck=1
repo_gpgcheck=1
enabled=1
skip_if_unavailable=True
sslverify=1
EOF
        )

        It 'should update successfully for regular cluster'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                fi
            End

            echo "${repo_config}" > "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is not set, use PMC repo'
            # repo config doesn't change
            The output should not include "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo is updated"
            # 1755216000 is the posix timestamp for 2025-08-15 00:00:00 UTC
            The output should include "tdnf mock called with args: --snapshottime 1755216000 update --exclude mshv-linuxloader --exclude kernel-mshv --repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo" should eq "${repo_config}"
        End

        It 'should update successfully for ni cluster'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                elif [[ "$@" == *"live-patching-repo-service"* ]]; then
                    echo "10.0.0.1"
                fi
            End

            echo "${repo_config}" > "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is: 10.0.0.1, use it to replace PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo is updated"
            # 1755216000 is the posix timestamp for 2025-08-15 00:00:00 UTC
            The output should include "tdnf mock called with args: --snapshottime 1755216000 update --exclude mshv-linuxloader --exclude kernel-mshv --repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo" should eq "${ni_repo_config}"
        End

        It 'should update successfully for ni cluster when repo service changes'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                elif [[ "$@" == *"live-patching-repo-service"* ]]; then
                    echo "10.0.0.2"
                fi
            End

            echo "${ni_repo_config}" > "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is: 10.0.0.2, use it to replace PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo is updated"
            # 1755216000 is the posix timestamp for 2025-08-15 00:00:00 UTC
            The output should include "tdnf mock called with args: --snapshottime 1755216000 update --exclude mshv-linuxloader --exclude kernel-mshv --repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo" should eq "${ni_repo_config_2}"
        End

        It 'should update successfully when cluster changed from ni to regular'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                fi
            End

            echo "${ni_repo_config}" > "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is not set, use PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo is updated"
            # 1755216000 is the posix timestamp for 2025-08-15 00:00:00 UTC
            The output should include "tdnf mock called with args: --snapshottime 1755216000 update --exclude mshv-linuxloader --exclude kernel-mshv --repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo" should eq "${repo_config}"
        End

        It 'should update successfully for custom cloud cluster'
            Mock kubectl
                if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                    echo "20250815T000000Z"
                elif [[ "$@" == *"live-patching-repo-service"* ]]; then
                    echo "10.0.0.1"
                fi
            End

            echo "${custom_cloud_repo_config}" > "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo"
      
            When run main
            The status should be success
            The output should include 'live patching repo service is: 10.0.0.1, use it to replace PMC repo'
            The output should include "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo is updated"
            # 1755216000 is the posix timestamp for 2025-08-15 00:00:00 UTC
            The output should include "tdnf mock called with args: --snapshottime 1755216000 update --exclude mshv-linuxloader --exclude kernel-mshv --repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
            The contents of file "${SECURITY_PATCH_REPO_DIR}/azurelinux-official-base.repo" should eq "${ni_repo_config_3}"
        End

        It 'should do nothing if golden timestamp is not set'
            Mock kubectl
                echo ""
            End
            When run main
            The status should be success
            The output should include 'golden timestamp is not set, skip live patching'
        End

        It 'should do nothing if golden timestamp equals current timestamp'
            Mock kubectl
                echo "20250820T000000Z"
            End
            When run main
            The status should be success
            The output should include 'golden timestamp is: 20250820T000000Z'
            The output should include 'current timestamp is: 20250820T000000Z'
            The output should include 'golden and current timestamp is the same, nothing to patch'
        End
    End
End
