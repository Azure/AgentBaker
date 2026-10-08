#!/bin/bash
# shellcheck disable=SC2329

Describe 'VHD Dalec sysext caching'
    Include './parts/linux/cloud-init/artifacts/cse_helpers.sh'
    BeforeAll "eval \"\$(sed -n '/^getLatestDalecSysextTag()/,/^}/p; /^cacheDalecSysextFromVersion()/,/^}/p' './vhdbuilder/packer/install-dependencies.sh')\""

    setup() {
        VHD_LOGS_FILEPATH=$(mktemp)
    }
    cleanup() {
        rm -f "${VHD_LOGS_FILEPATH}"
    }
    BeforeEach setup
    AfterEach cleanup

    getSystemdArch() { echo arm64; }
    retrycmd_silent() {
        shift 3
        "$@"
    }
    oras() {
        printf '%s\n' v1.33.4-9-azlinux3-arm64 v1.33.4-10-azlinux3-arm64 v1.33.4-99-azlinux3-x86-64
    }
    downloadSysextFromVersion() {
        echo "pull $*"
    }
    installSecureTLSBootstrapClientSysext() {
        echo "activate $*"
    }

    It 'selects the highest numeric revision for the requested architecture'
        When call getLatestDalecSysextTag mcr.microsoft.com/test v1.33.4 arm64
        The output should equal v1.33.4-10-azlinux3-arm64
        The status should be success
    End

    It 'fails clearly when no matching version exists'
        When call getLatestDalecSysextTag mcr.microsoft.com/test v1.32.4 arm64
        The status should equal 231
        The output should equal ""
        The error should include 'No matching Dalec sysext tag in mcr.microsoft.com/test for v1.32.4 (arm64)'
    End

    It 'does not select a tag published only for another architecture'
        oras() { echo v1.33.4-99-azlinux3-x86-64; }
        When call getLatestDalecSysextTag mcr.microsoft.com/test v1.33.4 arm64
        The status should equal 231
        The output should equal ""
        The error should include 'No matching Dalec sysext tag'
    End

    It 'does not accept partial output from a failed registry listing'
        oras() {
            echo v1.33.4-10-azlinux3-arm64
            return 1
        }
        When call getLatestDalecSysextTag mcr.microsoft.com/test v1.33.4 arm64
        The status should equal 231
        The output should equal ""
        The error should include 'Failed to list Dalec sysext tags from mcr.microsoft.com/test'
    End

    It 'rejects non-fixed versions at build time'
        When call getLatestDalecSysextTag mcr.microsoft.com/test v1.33 arm64
        The status should equal 231
        The error should include 'Invalid Dalec sysext version or architecture'
    End

    It 'tracks upstream versions in Renovate while ignoring artifact revisions'
        assert_renovate_versions() {
            python3 - <<'PY'
import json
import re

config = json.load(open(".github/renovate.json", encoding="utf-8"))
rule = next(rule for rule in config["packageRules"] if
            "extractVersion" in rule and
            "oss/v2/kubernetes/kubelet-sysext" in rule.get("matchPackageNames", []))
pattern = re.sub(r"\(\?<(\w+)>", r"(?P<\1>", rule["extractVersion"])
upstream = lambda tag: re.fullmatch(pattern, tag)["version"]
assert upstream("v1.34.11-9-azlinux3-x86-64") == upstream("v1.34.11-10-azlinux3-x86-64")
assert upstream("v1.34.11-10-azlinux3-x86-64") != upstream("v1.34.12-1-azlinux3-x86-64")
assert rule["versioning"] == "semver"
PY
        }
        When call assert_renovate_versions
        The status should be success
    End

    Describe 'component types'
        Parameters
            kubelet
            kubectl
            azure-acr-credential-provider
            aks-secure-tls-bootstrap-client
        End

        It "resolves and caches an exact immutable tag for $1"
            When call cacheDalecSysextFromVersion "$1" v1.33.4 mcr.microsoft.com/test:v1.33.4-arm64 /cache
            The status should be success
            The output should include "Resolved $1 sysext version v1.33.4 -> mcr.microsoft.com/test:v1.33.4-10-azlinux3-arm64"
            The output should include "pull $1 mcr.microsoft.com/test:v1.33.4-10-azlinux3-arm64 /cache"
            The contents of file "${VHD_LOGS_FILEPATH}" should include "mcr.microsoft.com/test:v1.33.4-10-azlinux3-arm64 (requested v1.33.4)"
        End
    End

    It 'activates the bootstrap client using the resolved revision rather than the tracked version'
        When call cacheDalecSysextFromVersion aks-secure-tls-bootstrap-client v1.33.4 mcr.microsoft.com/test:v1.33.4-arm64 /cache
        The status should be success
        The output should include 'activate v1.33.4-10-azlinux3'
    End

    It 'fails without downloading a floating tag when no match exists'
        When call cacheDalecSysextFromVersion kubelet v1.32.4 mcr.microsoft.com/test:v1.32.4-arm64 /cache
        The status should equal 231
        The output should equal ""
        The error should include 'No matching Dalec sysext tag'
        The contents of file "${VHD_LOGS_FILEPATH}" should equal ""
    End

    It 'fails without downloading when the registry listing fails'
        oras() { return 1; }
        When call cacheDalecSysextFromVersion kubelet v1.33.4 mcr.microsoft.com/test:v1.33.4-arm64 /cache
        The status should equal 231
        The output should equal ""
        The error should include 'Failed to list Dalec sysext tags'
    End

    It 'propagates pull errors without activating or logging a successfully cached artifact'
        downloadSysextFromVersion() { return 231; }
        When call cacheDalecSysextFromVersion aks-secure-tls-bootstrap-client v1.33.4 mcr.microsoft.com/test:v1.33.4-arm64 /cache
        The status should equal 231
        The output should include 'Resolved aks-secure-tls-bootstrap-client sysext version'
        The output should not include activate
        The contents of file "${VHD_LOGS_FILEPATH}" should equal ""
    End

    It 'rejects non-fixed versions at build time'
        When call cacheDalecSysextFromVersion kubelet v1.33 mcr.microsoft.com/test:v1.33-arm64 /cache
        The status should equal 231
        The output should equal ""
        The error should include 'Expected fixed vMAJOR.MINOR.PATCH'
    End

    Describe 'component manifest integration'
        BeforeAll "eval \"\$(sed -n '/^cachePackageAndBinaryComponents()/,/^}/p' './vhdbuilder/packer/install-dependencies.sh')\""

        setup_manifest() {
            COMPONENTS_FILEPATH=$(mktemp)
            jq '{Packages: [.Packages[] | select(.name == "kubelet" or .name == "kubectl" or .name == "azure-acr-credential-provider-pmc" or .name == "aks-secure-tls-bootstrap-client")]}' \
                parts/common/components.json > "${COMPONENTS_FILEPATH}"
            bootstrap_version=$(jq -r '.Packages[] | select(.name == "aks-secure-tls-bootstrap-client") | .downloadURIs.flatcar.current.versionsV2[0].latestVersion' "${COMPONENTS_FILEPATH}")
            kubelet_previous_version=$(jq -r '.Packages[] | select(.name == "kubelet") | .downloadURIs.flatcar.current.versionsV2[0].previousLatestVersion' "${COMPONENTS_FILEPATH}")
            OS_VERSION=current
            OS_VARIANT=""
            SYSTEMD_ARCH=arm64
            SCRIPT_NAME=install-dependencies
        }
        cleanup_manifest() {
            rm -f "${COMPONENTS_FILEPATH}"
        }
        BeforeEach setup_manifest
        AfterEach cleanup_manifest

        capture_benchmark() { :; }
        oras() {
            jq -r '.Packages[].downloadURIs.flatcar.current.versionsV2[] | .latestVersion, .previousLatestVersion | select(. != null) | . + "-10-azlinux3-arm64"' \
                "${COMPONENTS_FILEPATH}"
        }

        Parameters
            FLATCAR
            AZURECONTAINERLINUX
        End

        It "$1 caches all current and previous versions through the pinned resolver"
            OS=$1
            When call cachePackageAndBinaryComponents
            The status should be success
            The output should include 'pull kubelet mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:v'
            The output should include 'pull kubectl mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext:v'
            The output should include 'pull azure-acr-credential-provider mcr.microsoft.com/oss/v2/kubernetes/azure-acr-credential-provider-sysext:v'
            The output should include 'pull aks-secure-tls-bootstrap-client mcr.microsoft.com/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext:v'
            The output should include "activate ${bootstrap_version}-10-azlinux3"
            The contents of file "${VHD_LOGS_FILEPATH}" should include "sysext mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext:${kubelet_previous_version}-10-azlinux3-arm64"
        End
    End
End
