#!/bin/bash

Describe 'mariner-package-update.sh'
    setup_azurelinux3() {
        node_name="test-node"
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

    setup_mariner2() {
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
        BeforeEach 'setup_mariner2'

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
        BeforeEach 'setup_azurelinux3'

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

    Describe 'kubelet_update function'
        setup_kubelet_executable() {
            local version="${1:-1.29.10}"
            KUBELET_EXECUTABLE="${TEST_DIR}/kubelet"

            # Create a mock kubelet executable
            cat <<EOF > "${KUBELET_EXECUTABLE}"
#!/bin/bash
if [ "\$1" = "--version" ]; then
    echo "Kubernetes v${version}"
elif [ "\$1" = "--version=raw" ]; then
    echo "{\"major\":\"1\",\"minor\":\"29\",\"gitVersion\":\"v${version}\",\"gitCommit\":\"abc123\",\"gitTreeState\":\"clean\",\"buildDate\":\"2024-01-01T00:00:00Z\",\"goVersion\":\"go1.21.5\",\"compiler\":\"gc\",\"platform\":\"linux/amd64\"}"
fi
EOF
            chmod +x "${KUBELET_EXECUTABLE}"
        }

        setup_target_kubelet_version() {
            local target_version="$1"
            export target_kubelet_version="$target_version"

            Mock kubectl
                if [[ "$@" == *"live-patching-kubelet-version"* ]]; then
                    echo "$target_kubelet_version"
                fi
            End
        }

        Mock systemctl
            echo "systemctl mock called with args: $@"
        End

        Mock tdnf
            # Mock tdnf download for kubelet package
            if [[ "$@" == *"install"* ]] && [[ "$@" == *"kubelet"* ]]; then
                package_name=$(echo "$@" | grep -oP 'kubelet-[\d.]+')
                download_dir=$(echo "$@" | grep -oP -- '--downloaddir \K[^ ]+')
                echo "tdnf mock: downloading $package_name to $download_dir"
                # Create a mock RPM file
                mkdir -p "$download_dir/usr/bin"

                # Extract version from package name
                version=$(echo "$package_name" | sed 's/kubelet-//')

                # Create mock kubelet binary in the download dir structure
                cat <<KUBELET_EOF > "$download_dir/usr/bin/kubelet"
#!/bin/bash
if [ "\\\$1" = "--version" ]; then
    echo "Kubernetes v${version}"
elif [ "\\\$1" = "--version=raw" ]; then
    echo "{\"major\":\"1\",\"minor\":\"29\",\"gitVersion\":\"v${version}\",\"gitCommit\":\"def456\",\"gitTreeState\":\"clean\",\"buildDate\":\"2024-02-01T00:00:00Z\",\"goVersion\":\"go1.21.5\",\"compiler\":\"gc\",\"platform\":\"linux/amd64\"}"
fi
KUBELET_EOF
                chmod +x "$download_dir/usr/bin/kubelet"

                # Create a mock RPM file (just a marker)
                touch "$download_dir/${package_name}.x86_64.rpm"
                echo "Executed dnf install $package_name -y 1 times"
            else
                echo "tdnf mock called with args: $@"
            fi
        End

        Mock rpm2cpio
            # This is called to extract the RPM
            echo "rpm2cpio mock: extracting $@"
        End

        Mock cpio
            # Mock cpio extraction - files are already created by tdnf mock
            echo "cpio mock called with args: $@"
        End

        Describe 'on Mariner 2.0'
            BeforeEach 'setup_mariner2'

            It 'should skip kubelet update on Mariner 2.0'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version "1.29.11"

                When run kubelet_update
                The status should be success
                The output should include "kubelet patch is only supported on azurelinux 3.0, skipping kubelet update"
            End
        End

        Describe 'on AzureLinux 3.0'
            BeforeEach 'setup_azurelinux3'

            It 'should skip update when target version annotation is not set'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version ""

                When run kubelet_update
                The status should be success
                The output should include "target kubelet version is not set, skip kubelet update"
            End

            It 'should fail when kubelet executable is not found'
                KUBELET_EXECUTABLE="${TEST_DIR}/nonexistent-kubelet"
                setup_target_kubelet_version "1.29.11"

                When run kubelet_update
                The status should be failure
                The output should include "kubelet executable not found at ${KUBELET_EXECUTABLE}"
            End

            It 'should fail when major.minor version mismatch'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version "1.30.0"

                When run kubelet_update
                The status should be failure
                The output should include "current kubelet version is: 1.29.10"
                The output should include "kubelet major.minor version mismatch: current 1.29.10, target 1.30.0"
            End

            It 'should skip update when target version is older than current version'
                setup_kubelet_executable "1.29.12"
                setup_target_kubelet_version "1.29.10"

                When run kubelet_update
                The status should be success
                The output should include "current kubelet version is: 1.29.12"
                The output should include "target kubelet version to update to is: 1.29.10"
                The output should include "Skip kubelet update since target_kubelet_version (1.29.10) is older than current_kubelet_version (1.29.12)"
            End

            It 'should skip update when binary sha256 is the same'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version "1.29.10"
                export KUBELET_EXECUTABLE

                # Mock tdnf to copy the same kubelet binary
                Mock tdnf
                    if [[ "$@" == *"install"* ]] && [[ "$@" == *"kubelet"* ]]; then
                        download_dir=$(echo "$@" | grep -oP -- '--downloaddir \K[^ ]+')
                        mkdir -p "$download_dir/usr/bin"
                        # Copy the same kubelet to simulate same binary
                        cp "${KUBELET_EXECUTABLE}" "$download_dir/usr/bin/kubelet"
                        touch "$download_dir/kubelet-1.29.10.x86_64.rpm"
                        echo "Executed dnf install kubelet-1.29.10 -y 1 times"
                    fi
                End

                When run kubelet_update
                The status should be success
                The output should include "kubelet binary is the same, no need to update"
            End

            It 'should successfully update kubelet to newer patch version'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version "1.29.11"

                When run kubelet_update
                The status should be success
                The output should include "target kubelet version to update to is: 1.29.11"
                The output should include "current kubelet version is: 1.29.10"
                The output should include "updating kubelet from 1.29.10"
                The output should include "to version 1.29.11"
                The output should include "systemctl mock called with args: restart kubelet.service"
                The output should include "kubelet update completed successfully"
            End

            It 'should successfully update kubelet with same version but different release'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version "1.29.10"

                # Mock tdnf to provide a different binary (different sha256)
                Mock tdnf
                    if [[ "$@" == *"install"* ]] && [[ "$@" == *"kubelet"* ]]; then
                        download_dir=$(echo "$@" | grep -oP -- '--downloaddir \K[^ ]+')
                        mkdir -p "$download_dir/usr/bin"
                        # Create a different binary content with a comment to make sha256 different
                        cat <<KUBELET_EOF > "$download_dir/usr/bin/kubelet"
#!/bin/bash
# This is a different kubelet binary with different sha256
if [ "\\\$1" = "--version" ]; then
    echo "Kubernetes v1.29.10"
elif [ "\\\$1" = "--version=raw" ]; then
    echo "{\"major\":\"1\",\"minor\":\"29\",\"gitVersion\":\"v1.29.10\",\"gitCommit\":\"xyz789\",\"gitTreeState\":\"clean\",\"buildDate\":\"2024-03-01T00:00:00Z\",\"goVersion\":\"go1.21.5\",\"compiler\":\"gc\",\"platform\":\"linux/amd64\"}"
fi
KUBELET_EOF
                        chmod +x "$download_dir/usr/bin/kubelet"
                        touch "$download_dir/kubelet-1.29.10.x86_64.rpm"
                        echo "Executed dnf install kubelet-1.29.10 -y 1 times"
                    fi
                End

                When run kubelet_update
                The status should be success
                The output should include "updating kubelet from 1.29.10"
                The output should include "to version 1.29.10"
                The output should include "kubelet update completed successfully"
            End

            It 'should fail when downloaded kubelet binary is not found'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version "1.29.11"

                # Mock tdnf to not create the kubelet binary
                Mock tdnf
                    if [[ "$@" == *"install"* ]] && [[ "$@" == *"kubelet"* ]]; then
                        download_dir=$(echo "$@" | grep -oP -- '--downloaddir \K[^ ]+')
                        mkdir -p "$download_dir"
                        # Don't create the usr/bin/kubelet file
                        touch "$download_dir/kubelet-1.29.11.x86_64.rpm"
                        echo "Executed dnf install kubelet-1.29.11 -y 1 times"
                    fi
                End

                When run kubelet_update
                The status should be failure
                The output should include "kubelet binary not found in the downloaded package"
            End

            It 'should update from 1.29.10 to 1.29.15'
                setup_kubelet_executable "1.29.10"
                setup_target_kubelet_version "1.29.15"

                When run kubelet_update
                The status should be success
                The output should include "target kubelet version to update to is: 1.29.15"
                The output should include "current kubelet version is: 1.29.10"
                The output should include "updating kubelet from 1.29.10"
                The output should include "to version 1.29.15"
                The output should include "systemctl mock called with args: restart kubelet.service"
                The output should include "kubelet update completed successfully"
            End

            It 'should handle version comparison correctly for multi-digit patch versions'
                setup_kubelet_executable "1.29.9"
                setup_target_kubelet_version "1.29.10"

                When run kubelet_update
                The status should be success
                The output should include "updating kubelet from 1.29.9"
                The output should include "to version 1.29.10"
                The output should include "kubelet update completed successfully"
            End
        End
    End
End
