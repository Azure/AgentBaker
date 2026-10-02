#!/bin/bash

Describe 'Dalec sysext tag resolution'
    Include './parts/linux/cloud-init/artifacts/cse_helpers.sh'

    retrycmd_silent() {
        shift 3
        "$@"
    }

    oras() {
        printf '%s\n' \
            v1.33.4-9-azlinux3-x86-64 \
            v1.33.4-10-azlinux3-x86-64 \
            v1.33.4-2-azlinux3-arm64 \
            v1.33.4-11-azlinux3-arm64 \
            v1.33.40-99-azlinux3-x86-64 \
            v1.33.5-99-azlinux3-x86-64 \
            v1.33.4-99-azlinux2-x86-64 \
            v1.33.4-preview-azlinux3-x86-64 \
            v1.33.4-999-azlinux3-x86-64-extra \
            v1.33.4-azlinux3-x86-64 \
            v1.33.4
    }

    Describe 'matching versions and architectures'
        Parameters
            v1.33.4 x86-64 v1.33.4-10-azlinux3-x86-64
            1.33.4 arm64 v1.33.4-11-azlinux3-arm64
            1.33 x86-64 v1.33.40-99-azlinux3-x86-64
        End

        It "selects the numerically latest matching tag for $1 $2"
            When call getLatestDalecSysextTag mcr.microsoft.com/test "$1" "$2"
            The output should equal "$3"
            The status should be success
        End
    End

    It 'fails clearly when no matching version exists, including under VHD strict shell options'
        resolve_strict() {
            set -euo pipefail
            getLatestDalecSysextTag mcr.microsoft.com/test v1.32.4 x86-64
        }
        When run resolve_strict
        The status should equal 231
        The output should equal ""
        The error should include 'No matching Dalec sysext tag in mcr.microsoft.com/test for v1.32.4 (x86-64)'
    End

    It 'fails when only another architecture is published'
        oras() { echo v1.33.4-10-azlinux3-x86-64; }
        When call getLatestDalecSysextTag mcr.microsoft.com/test v1.33.4 arm64
        The status should equal 231
        The output should equal ""
        The error should include 'No matching Dalec sysext tag'
    End

    It 'does not accept partial output from a failed registry listing'
        oras() {
            echo v1.33.4-10-azlinux3-x86-64
            return 1
        }
        When call getLatestDalecSysextTag mcr.microsoft.com/test v1.33.4 x86-64
        The status should equal 231
        The output should equal ""
        The error should include 'Failed to list Dalec sysext tags from mcr.microsoft.com/test'
    End

    Describe 'invalid inputs'
        Parameters
            '1.33.*' x86-64
            v1.33.4 amd64
        End

        It "rejects invalid input $1 $2"
            When call getLatestDalecSysextTag mcr.microsoft.com/test "$1" "$2"
            The status should equal 231
            The error should include 'Invalid Dalec sysext version or architecture'
        End
    End

    Describe 'CSE consumers'
        Parameters
            flatcar
            acl
        End

        BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""
        match_remote_for_distro() {
            local distro=$1
            shift
            source "./parts/linux/cloud-init/artifacts/${distro}/cse_install_${distro}.sh" >/dev/null
            matchRemoteSysext "$@"
        }

        It "$1 uses the same exact patch and numeric revision selector"
            When call match_remote_for_distro "$1" mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext 1.33.4 x86-64
            The output should equal v1.33.4-10-azlinux3-x86-64
            The status should be success
        End

        It "$1 preserves the isolated-registry revision-1 workaround"
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=registry.example
            When call match_remote_for_distro "$1" registry.example/kubelet-sysext 1.33.4 arm64
            The output should equal v1.33.4-1-azlinux3-arm64
            The status should be success
        End

        It "$1 refuses a remote tag returned with a failure status"
            merge_remote_failure_for_distro() {
                source "./parts/linux/cloud-init/artifacts/$1/cse_install_$1.sh" >/dev/null
                matchLocalSysext() { echo /nonexistent/sysext.raw; }
                matchRemoteSysext() {
                    echo v1.33.4-10-azlinux3-x86-64
                    return 231
                }
                downloadSysextFromVersion() { echo unexpected-download; }
                mergeSysexts kubelet mcr.microsoft.com/test 1.33.4
            }
            When call merge_remote_failure_for_distro "$1"
            The status should equal 231
            The output should include 'Failed to find valid kubelet system extension for 1.33.4 remotely'
            The output should not include unexpected-download
        End
    End

    Describe 'ACL GPU sysext compatibility'
        Include './parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh'
        BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""

        It 'still selects the exact OS image version tag'
            oras() {
                printf '%s\n' 3.0.20260304 3.0.20260305
            }
            When call matchRemoteSysext mcr.microsoft.com/azurelinux/3.0/azure-container-linux/nvidia-driver-cuda 3.0.20260304 x86-64
            The output should equal 3.0.20260304
            The status should be success
        End
    End
End
