#!/bin/bash

# Mock functions that the script depends on
oras() {
    echo "mock oras $*"
}

ln() {
    echo "mock ln $*"
}

systemd-sysext() {
    echo "mock systemd-sysext $*"
}

timeout() {
    shift # remove timeout duration
    "$@" # execute the command
}

mkdir() {
    echo "mock mkdir $*"
}

getSystemdArch() {
    echo "x86-64"
}

# Mock exit to prevent actual exits during tests
exit() {
    echo "mock exit $1"
    return "$1"
}

Describe 'cse_install_flatcar.sh'
    Include "./parts/linux/cloud-init/artifacts/flatcar/cse_install_flatcar.sh"

    Describe 'downloadSysextFromVersion'
        It 'downloads sysext with default download directory'
            # Mock successful oras pull
            oras() {
                case "$1" in
                    "pull")
                        [ "$2" = "--output" ] && [ "$3" = "/opt/kubelet/downloads" ] && [ "$4" = "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.28.101-1-azlinux3-x86-64" ]
                        return 0
                        ;;
                    *)
                        return 1
                        ;;
                esac
            }
            When call downloadSysextFromVersion "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.28.101-1-azlinux3-x86-64"
            The output should include "Succeeded to download kubelet from mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.28.101-1-azlinux3-x86-64"
            The status should be success
        End

        It 'downloads sysext with custom download directory'
            # Mock successful oras pull
            oras() {
                case "$1" in
                    "pull")
                        [ "$2" = "--output" ] && [ "$3" = "/custom/path" ] && [ "$4" = "mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext:v1.28.101-1-azlinux3-x86-64" ]
                        return 0
                        ;;
                    *)
                        return 1
                        ;;
                esac
            }
            When call downloadSysextFromVersion "kubectl" "mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext:v1.28.101-1-azlinux3-x86-64" "/custom/path"
            The output should include "Succeeded to download kubectl from mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext:v1.28.101-1-azlinux3-x86-64"
            The status should be success
        End

        It 'retries on failure and eventually succeeds'
            local call_count=0
            # Mock oras to fail twice then succeed
            oras() {
                call_count=$((call_count + 1))
                case "$1" in
                    "pull")
                        if [ $call_count -le 2 ]; then
                            return 1
                        else
                            return 0
                        fi
                        ;;
                    *)
                        return 1
                        ;;
                esac
            }
            sleep() {
                echo "sleeping $1 seconds"
            }
            When call downloadSysextFromVersion "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.28.101-1-azlinux3-x86-64"
            The output should include "sleeping 5 seconds"
            The output should include "Succeeded to download kubelet"
            The status should be success
        End

        # Note: Testing retry/exit behavior is complex in shell tests due to exit() mocking limitations
        # The function uses exit() which cannot be easily mocked without redefining the entire function
        # The three tests above cover the main success scenarios and retry logic
    End

    Describe 'mergeSysexts'
        It 'creates symlinks and refreshes systemd-sysext for single sysext'
            ln() {
                echo "ln $*"
            }
            systemd-sysext() {
                echo "systemd-sysext $*"
            }
            printf() {
                # Mock printf for specific glob patterns, fall back to real printf for everything else
                case $2 in
                    /opt/kubelet/downloads/*) echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw" ;;
                    *) ( unset printf; printf "${@}" ) ;;
                esac
            }
            # Mock test to make the file check pass
            test() {
                if [ "$1" = "-f" ]; then
                    return 0
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "1.33"
            The output should include "ln -snf /opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw /etc/extensions/kubelet.raw"
            The output should include "systemd-sysext --no-reload refresh"
            The status should be success
        End

        It 'selects highest version when multiple matches exist'
            ln() {
                echo "ln $*"
            }
            systemd-sysext() {
                echo "systemd-sysext $*"
            }
            printf() {
                # Mock printf for glob patterns to return multiple versions for sort testing
                case $2 in
                    /opt/kubelet/downloads/*) cat << 'EOF'
/opt/kubelet/downloads/kubelet-v1.33.2-1-azlinux3-x86-64.raw
/opt/kubelet/downloads/kubelet-v1.33.10-1-azlinux3-x86-64.raw
/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw
EOF
;;
                    *) ( unset printf; printf "${@}" ) ;;
                esac
            }
            test() {
                if [ "$1" = "-f" ]; then
                    return 0
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "1.33"
            # Should select v1.33.10 (highest version) due to sort -V | tail -n1
            The output should include "ln -snf /opt/kubelet/downloads/kubelet-v1.33.10-1-azlinux3-x86-64.raw /etc/extensions/kubelet.raw"
            The output should include "systemd-sysext --no-reload refresh"
            The status should be success
        End

        It 'creates symlinks for multiple sysexts'
            ln() {
                echo "ln $*"
            }
            systemd-sysext() {
                echo "systemd-sysext $*"
            }
            printf() {
                # Mock printf to return different paths based on the sysext type
                case "$2" in
                    /opt/kubelet/downloads/*) echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw" ;;
                    /opt/kubectl/downloads/*) echo "/opt/kubectl/downloads/kubectl-v1.33.4-1-azlinux3-x86-64.raw" ;;
                    *) ( unset printf; printf "${@}" ) ;;
                esac
            }
            test() {
                if [ "$1" = "-f" ]; then
                    return 0
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "1.33" "kubectl" "1.33"
            The output should include "ln -snf /opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw /etc/extensions/kubelet.raw"
            The output should include "ln -snf /opt/kubectl/downloads/kubectl-v1.33.4-1-azlinux3-x86-64.raw /etc/extensions/kubectl.raw"
            The output should include "systemd-sysext --no-reload refresh"
            The status should be success
        End

        It 'handles missing sysext files gracefully'
            printf() {
                # Mock printf for the missing file test case
                case "$2" in
                    /opt/kubelet/downloads/*) echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw" ;;
                    *) ( unset printf; printf "${@}" ) ;;
                esac
            }
            # Mock test to make the file check fail
            test() {
                if [ "$1" = "-f" ]; then
                    return 1
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "1.33"
            The output should include "Failed to find valid kubelet version for 1.33"
            The output should include "mock exit 1"
        End

        It 'calls systemd-sysext refresh even with no arguments'
            systemd-sysext() {
                echo "systemd-sysext $*"
            }
            When call mergeSysexts
            The output should include "systemd-sysext --no-reload refresh"
            The status should be success
        End
    End

    Describe 'installKubeletKubectlFromPkg'
        It 'calls mergeSysexts with kubelet and kubectl using base version'
            mergeSysexts() {
                echo "mergeSysexts called with: $*"
            }
            ln() {
                echo "ln $*"
            }
            When call installKubeletKubectlFromPkg "1.33"
            The output should include "mergeSysexts called with: kubelet 1.33 kubectl 1.33"
            The output should include "ln -snf /usr/bin/kubelet /usr/bin/kubectl /opt/bin/"
            The status should be success
        End
    End

    Describe 'installStandaloneContainerd'
        It 'reports current containerd version and shows expected behavior'
            containerd() {
                echo "containerd github.com/containerd/containerd v1.7.0"
            }
            systemctl() {
                echo "systemctl $*"
            }
            mkdir() {
                echo "mkdir $*"
            }
            cp() {
                echo "cp $*"
            }
            When call installStandaloneContainerd "v1.6.0"
            The output should include "currently installed containerd version: v1.7.0. Desired version v1.6.0. Skipping installStandaloneContainerd on Flatcar."
            The status should be success
        End
    End

    Describe 'cleanUpGPUDrivers'
        It 'removes GPU directories'
            rm() {
                echo "rm $*"
            }
            GPU_DEST="/opt/gpu"
            When call cleanUpGPUDrivers
            The output should include "rm -Rf /opt/gpu /opt/gpu"
            The status should be success
        End
    End
End
