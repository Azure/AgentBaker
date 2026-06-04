#!/bin/bash

# Mock functions that the ACL script depends on
oras() {
    echo "mock oras $*" >&2
}

ln() {
    echo "mock ln $*" >&2
}

systemd-sysext() {
    echo "mock systemd-sysext $*" >&2
}

timeout() {
    shift # remove timeout duration
    "$@" # execute the command
}

mkdir() {
    echo "mock mkdir $*" >&2
}

getSystemdArch() {
    echo "x86-64"
}

getCPUArch() {
    echo "amd64"
}

sleep() {
    echo "sleeping $1 seconds" >&2
}

find() {
    echo "mock find $*" >&2
}

CSE_STARTTIME_SECONDS=$(date +%s)

Describe 'cse_install_acl.sh'
    Include "./parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    Describe 'installSecureTLSBootstrapClientSysext'
        It 'calls mergeSysexts with correct URL and creates symlink on success'
            mergeSysexts() {
                echo "mock mergeSysexts $*" >&2
            }
            ln() {
                echo "mock ln $*" >&2
            }
            When call installSecureTLSBootstrapClientSysext "1.1.3"
            The error should include "mock mergeSysexts aks-secure-tls-bootstrap-client mcr.microsoft.com/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext 1.1.3"
            The error should include "mock ln -snf /usr/bin/aks-secure-tls-bootstrap-client /opt/bin/aks-secure-tls-bootstrap-client"
            The status should be success
        End

        It 'uses custom registry when provided'
            mergeSysexts() {
                echo "mock mergeSysexts $*" >&2
            }
            ln() {
                echo "mock ln $*" >&2
            }
            When call installSecureTLSBootstrapClientSysext "1.1.3" "custom.registry.io"
            The error should include "mock mergeSysexts aks-secure-tls-bootstrap-client custom.registry.io/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext 1.1.3"
            The status should be success
        End

        It 'returns ERR_ORAS_PULL_SYSEXT_FAIL when mergeSysexts fails'
            mergeSysexts() {
                return 1
            }
            ERR_ORAS_PULL_SYSEXT_FAIL=231
            When call installSecureTLSBootstrapClientSysext "1.1.3"
            The output should include "Failed to install aks-secure-tls-bootstrap-client sysext"
            The status should be failure
        End

        It 'strips a leading v from the version before passing to mergeSysexts'
            mergeSysexts() {
                echo "mock mergeSysexts $*" >&2
            }
            ln() {
                echo "mock ln $*" >&2
            }
            When call installSecureTLSBootstrapClientSysext "v1.1.3-2-azlinux3"
            The error should include "mock mergeSysexts aks-secure-tls-bootstrap-client mcr.microsoft.com/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext 1.1.3-2-azlinux3"
            The error should not include "vv1.1.3"
            The status should be success
        End
    End
End
