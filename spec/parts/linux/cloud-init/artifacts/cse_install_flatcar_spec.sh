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

Describe 'cse_install_flatcar.sh'
    Include "./parts/linux/cloud-init/artifacts/flatcar/cse_install_flatcar.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

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
            The output should include "Succeeded to download kubelet system extension from mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.28.101-1-azlinux3-x86-64"
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
            The output should include "Succeeded to download kubectl system extension from mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext:v1.28.101-1-azlinux3-x86-64"
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
            The output should include "Succeeded to download kubelet system extension"
            The status should be success
        End

        It 'returns error code when all retries fail'
            # Mock oras to always fail
            oras() {
                return 1
            }
            sleep() {
                echo "sleeping $1 seconds"
            }
            When call downloadSysextFromVersion "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.28.101-1-azlinux3-x86-64"
            The output should include "sleeping 5 seconds"
            The output should include "Failed to download kubelet system extension from mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.28.101-1-azlinux3-x86-64"
            The status should be failure
        End
    End

    Describe 'matchLocalSysext'
        It 'finds matching local sysext file'
            printf() {
                case $2 in
                    /opt/kubelet/downloads/*) echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw" ;;
                    *) ( unset printf; printf "${@}" ) ;;
                esac
            }
            When call matchLocalSysext "kubelet" "1.33" "x86-64"
            The output should equal "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw"
            The status should be success
        End

        It 'returns highest version when multiple matches exist'
            printf() {
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
            When call matchLocalSysext "kubelet" "1.33" "x86-64"
            The output should equal "/opt/kubelet/downloads/kubelet-v1.33.10-1-azlinux3-x86-64.raw"
            The status should be success
        End
    End

    Describe 'matchRemoteSysext'
        It 'finds matching remote sysext tag'
            oras() {
                case "$1" in
                    "repo")
                        [ "$2" = "tags" ] && echo "v1.33.4-1-azlinux3-x86-64"
                        return 0
                        ;;
                    *)
                        return 1
                        ;;
                esac
            }
            grep() {
                case "$2" in
                    "v1\\.33[.~-].*-azlinux3-x86-64") echo "v1.33.4-1-azlinux3-x86-64" ;;
                    *) ( unset grep; grep "${@}" ) ;;
                esac
            }
            When call matchRemoteSysext "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33" "x86-64"
            The output should equal "v1.33.4-1-azlinux3-x86-64"
            The status should be success
        End

        It 'returns highest version when multiple remote matches exist'
            oras() {
                case "$1" in
                    "repo")
                        [ "$2" = "tags" ] && cat << 'EOF'
v1.33.2-1-azlinux3-x86-64
v1.33.10-1-azlinux3-x86-64
v1.33.4-1-azlinux3-x86-64
EOF
                        return 0
                        ;;
                    *)
                        return 1
                        ;;
                esac
            }
            grep() {
                case "$2" in
                    "v1\\.33[.~-].*-azlinux3-x86-64") cat << 'EOF'
v1.33.2-1-azlinux3-x86-64
v1.33.10-1-azlinux3-x86-64
v1.33.4-1-azlinux3-x86-64
EOF
;;
                    *) ( unset grep; grep "${@}" ) ;;
                esac
            }
            When call matchRemoteSysext "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33" "x86-64"
            The output should equal "v1.33.10-1-azlinux3-x86-64"
            The status should be success
        End

        It 'retries on failure and eventually succeeds'
            # A file is needed to track the call count because the oras call is
            # piped, which would cause the variable change to be lost.
            local call_count_file=$(mktemp)
            # Mock oras to fail twice then succeed
            oras() {
                local call_count=$(($(< "$call_count_file") + 1))
                echo "$call_count" > "$call_count_file"
                case "$1" in
                    "repo")
                        if [ $call_count -le 2 ]; then
                            return 1  # Fail first two attempts
                        else
                            [ "$2" = "tags" ] && echo "v1.33.4-1-azlinux3-x86-64"
                            return 0  # Succeed on third attempt
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
            When call matchRemoteSysext "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33" "x86-64"
            The output should include "sleeping 5 seconds"
            The output should include "v1.33.4-1-azlinux3-x86-64"
            The status should be success
        End

        It 'returns failure when all retries exhausted'
            # Mock oras to always fail
            oras() {
                return 1
            }
            sleep() {
                echo "sleeping $1 seconds"
            }
            When call matchRemoteSysext "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33" "x86-64"
            The output should include "sleeping 5 seconds"
            The status should be failure
        End
    End

    Describe 'mergeSysexts'
        It 'creates symlinks and refreshes systemd-sysext for single sysext when file exists locally'
            ln() {
                echo "ln $*"
            }
            systemd-sysext() {
                echo "systemd-sysext $*"
            }
            matchLocalSysext() {
                echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw"
            }
            # Mock test to make the file check pass
            test() {
                if [ "$1" = "-f" ]; then
                    return 0
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33"
            The output should include "ln -snf /opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw /etc/extensions/kubelet.raw"
            The output should include "systemd-sysext --no-reload refresh"
            The status should be success
        End

        It 'downloads and creates symlinks when file does not exist locally but exists remotely'
            ln() {
                echo "ln $*"
            }
            systemd-sysext() {
                echo "systemd-sysext $*"
            }
            matchLocalSysext() {
                echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw"
            }
            matchRemoteSysext() {
                echo "v1.33.4-1-azlinux3-x86-64"
            }
            downloadSysextFromVersion() {
                echo "downloadSysextFromVersion $*"
                return 0
            }
            # Mock test to fail first time (local), then succeed second time (after download)
            local test_call_count=0
            test() {
                if [ "$1" = "-f" ]; then
                    test_call_count=$((test_call_count + 1))
                    if [ $test_call_count -eq 1 ]; then
                        return 1  # First call: file doesn't exist locally
                    else
                        return 0  # Second call: file exists after download
                    fi
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33"
            The output should include "Failed to find valid kubelet system extension for 1.33 locally"
            The output should include "downloadSysextFromVersion kubelet mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.33.4-1-azlinux3-x86-64"
            The output should include "ln -snf /opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw /etc/extensions/kubelet.raw"
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
            matchLocalSysext() {
                case "$1" in
                    "kubelet") echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw" ;;
                    "kubectl") echo "/opt/kubectl/downloads/kubectl-v1.33.4-1-azlinux3-x86-64.raw" ;;
                esac
            }
            test() {
                if [ "$1" = "-f" ]; then
                    return 0
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33" "kubectl" "mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext" "1.33"
            The output should include "ln -snf /opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw /etc/extensions/kubelet.raw"
            The output should include "ln -snf /opt/kubectl/downloads/kubectl-v1.33.4-1-azlinux3-x86-64.raw /etc/extensions/kubectl.raw"
            The output should include "systemd-sysext --no-reload refresh"
            The status should be success
        End

        It 'handles missing sysext files gracefully when not found locally or remotely'
            matchLocalSysext() {
                echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw"
            }
            matchRemoteSysext() {
                echo ""  # Return empty string to simulate no remote match
            }
            # Mock test to make the file check fail
            test() {
                if [ "$1" = "-f" ]; then
                    return 1
                fi
                return 1
            }
            When call mergeSysexts "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33"
            The output should include "Failed to find valid kubelet system extension for 1.33 locally"
            The output should include "Failed to find valid kubelet system extension for 1.33 remotely"
            The status should be failure
        End

        It 'handles download failure gracefully'
            matchLocalSysext() {
                echo "/opt/kubelet/downloads/kubelet-v1.33.4-1-azlinux3-x86-64.raw"
            }
            matchRemoteSysext() {
                echo "v1.33.4-1-azlinux3-x86-64"
            }
            downloadSysextFromVersion() {
                echo "downloadSysextFromVersion $*"
                return 1  # Simulate download failure
            }
            # Mock test to make the file check fail (file doesn't exist locally)
            test() {
                return 1
            }
            When call mergeSysexts "kubelet" "mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext" "1.33"
            The output should include "Failed to find valid kubelet system extension for 1.33 locally"
            The output should include "downloadSysextFromVersion kubelet mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v1.33.4-1-azlinux3-x86-64"
            The status should be failure
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
        It 'calls mergeSysexts with kubelet and kubectl URLs and creates symlinks on success'
            mergeSysexts() {
                echo "mergeSysexts called with: $*"
                return 0
            }
            ln() {
                echo "ln $*"
            }
            When call installKubeletKubectlFromPkg "1.33"
            The output should include "mergeSysexts called with: kubelet mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext 1.33 kubectl mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext 1.33"
            The output should include "ln -snf /usr/bin/kubelet /usr/bin/kubectl /opt/bin/"
            The status should be success
        End

        It 'falls back to installKubeletKubectlFromURL when mergeSysexts fails'
            mergeSysexts() {
                echo "mergeSysexts called with: $*"
                return 1  # Simulate failure
            }
            installKubeletKubectlFromURL() {
                echo "installKubeletKubectlFromURL called"
            }
            When call installKubeletKubectlFromPkg "1.33"
            The output should include "mergeSysexts called with: kubelet mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext 1.33 kubectl mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext 1.33"
            The output should include "installKubeletKubectlFromURL called"
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
