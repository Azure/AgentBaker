#!/bin/bash
lsb_release() {
    echo "mock lsb_release"
}

readPackage() {
    local packageName=$1
    package=$(jq ".Packages" "spec/parts/linux/cloud-init/artifacts/test_components.json" | jq ".[] | select(.name == \"$packageName\")")
    echo "$package"
}

readContainerImage() {
    local containerImageName=$1
    containerImage=$(jq ".ContainerImages" "spec/parts/linux/cloud-init/artifacts/test_components.json" | jq ".[] | select(.downloadURL | contains(\"$containerImageName\"))")
    echo "$containerImage"
}

Describe 'cse_helpers.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'updatePackageVersions'
        It 'returns downloadURIs.ubuntu.r2204.versionsV2 of package pkgVersionsV2 for UBUNTU 22.04'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "UBUNTU" "22.04"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion1 dummyVersion0.9"
        End
        It 'returns downloadURIs.ubuntu.r2004.versionsV2 of package pkgVersionsV2 for UBUNTU 20.04'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion2"
        End
        It 'returns downloadURIs.mariner.current.versionsV2 of package pkgVersionsV2 for MARINER current'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "MARINER" "current"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion5 dummyVersion6.1 dummyVersion6.0"
        End
        It 'returns <SKIP> if there is a <SKIP> in latestVersion'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "MARINERKATA" "current"
            The variable PACKAGE_VERSIONS[@] should equal "<SKIP>"
        End
        It 'returns downloadURIs.default.current.versionsV2 of package pkgVersionsV2 for unknown OS distro'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "unknownOS" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion7"
        End
        It 'returns downloadURIs.ubuntu.current.versionsV2 of package pkgVersionsV2 for UBUNTU unkown release'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "UBUNTU" "unknown_release"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersionCurrent"
        End
        It 'returns downloadURIs.default.current.versionsV2 of package fake-runc for MARINER unkown release'
            package=$(readPackage "fake-runc")
            When call updatePackageVersions "$package" "MARINER" "unknown_release"
            The variable PACKAGE_VERSIONS[@] should be undefined
        End
        It 'returns downloadURIs.default.current.versions of package pkgVersions for default.current as a fallback case'
            package=$(readPackage "pkgVersions")
            When call updatePackageVersions "$package" "default" "current"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersionFallback1.1 dummyVersionFallback1.0"
        End
    End

    Describe 'updatePackageDownloadURL'
        It 'returns downloadURIs.ubuntu.r2204.downloadURL of package pkgVersionsV2 for ubuntu.r2204'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageDownloadURL "$package" "UBUNTU" "22.04"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://dummypath/v\${version}/dummy_\${version}_linux_\${CPU_ARCH}.tar.gz"
        End
        It 'returns downloadURIs.ubuntu.current.downloadURL of package pkgVersionsV2 for ubuntu unknown release'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageDownloadURL "$package" "UBUNTU" "dummy_release"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://dummydefaultcurrentpath/v\${version}/dummy_\${version}_linux_\${CPU_ARCH}.tar.gz"
        End
    End

    Describe 'evalPackageDownloadURL'
        It 'returns empty string for empty downloadURL'
            When call evalPackageDownloadURL ""
            The output should equal ""
        End
        It 'returns evaluated downloadURL of package dummypath'
            version="0.0.1"
            CPU_ARCH="amd64"
            When call evalPackageDownloadURL "https://dummypath/v\${version}/binaries/dummypath-linux-\${CPU_ARCH}-v\${version}.tgz"
            The output should equal 'https://dummypath/v0.0.1/binaries/dummypath-linux-amd64-v0.0.1.tgz'
        End
    End

    Describe 'update_base_url'
        It 'updates base url to packages.aks.azure.com when PACKAGE_DOWNLOAD_BASE_URL is packages.aks.azure.com and base url is acs-mirror.azureedge.net'
            PACKAGE_DOWNLOAD_BASE_URL="packages.aks.azure.com"
            When call update_base_url "https://acs-mirror.azureedge.net/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz"
            The output should equal "https://packages.aks.azure.com/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz"
        End
        It 'updates base url to acs-mirror.azureedge.net when PACKAGE_DOWNLOAD_BASE_URL is acs-mirror.azureedge.net and base url is packages.aks.azure.com'
            PACKAGE_DOWNLOAD_BASE_URL="acs-mirror.azureedge.net"
            When call update_base_url "https://packages.aks.azure.com/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz"
            The output should equal "https://acs-mirror.azureedge.net/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz"
        End
        It 'does not change URL when base is not acs-mirror.azureedge.net or packages.aks.azure.com'
            PACKAGE_DOWNLOAD_BASE_URL="packages.aks.azure.com"
            When call update_base_url "mcr.microsoft.com/oss/binaries/kubernetes/kubernetes-node:v1.27.102-akslts-linux-arm64"
            The output should equal "mcr.microsoft.com/oss/binaries/kubernetes/kubernetes-node:v1.27.102-akslts-linux-arm64"
        End
    End

    Describe 'resolve_packages_source_url'
        It 'sets PACKAGE_DOWNLOAD_BASE_URL to packages.aks.azure.com when run locally'
            # Mock the curl command to simulate a successful response instead of making an actual network call
            curl() {
                echo 200
            }
            When call resolve_packages_source_url
            The output should equal "Established connectivity to packages.aks.azure.com."
            The variable PACKAGE_DOWNLOAD_BASE_URL should equal "packages.aks.azure.com"
        End
    End

    Describe 'pkgVersionsV2'
        It 'returns release version r2004 for package pkgVersionsV2 in UBUNTU 20.04'
            package=$(readPackage "pkgVersionsV2")
            os="UBUNTU"
            osVersion="20.04"
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r2004\""
        End
        It 'returns release version current for package pkgVersionsV2 in Mariner.uknown_release'
            package=$(readPackage "pkgVersionsV2")
            os="MARINER"
            osVersion="uknown_release"
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "current"
        End
    End

    Describe 'updateMultiArchVersions'
        It 'returns multiArchVersionsV2 for containerImage mcr.microsoft.com/dummyImageWithMultiArchVersionsV2'
            containerImage=$(readContainerImage "mcr.microsoft.com/dummyImageWithMultiArchVersionsV2")
            When call updateMultiArchVersions "$containerImage"
            The variable MULTI_ARCH_VERSIONS[@] should equal "dummyVersion1.1 dummyVersion2.1 dummyVersion1 dummyVersion2"
        End
        It 'returns multiArchVersions for containerImage mcr.microsoft.com/dummyImageWithOldMultiArchVersions'
            containerImage=$(readContainerImage "mcr.microsoft.com/dummyImageWithOldMultiArchVersions")
            When call updateMultiArchVersions "$containerImage"
            The variable MULTI_ARCH_VERSIONS[@] should equal "dummyVersion3 dummyVersion4 dummyVersion5"
        End
        It 'returns multiArchVersions for containerImage mcr.microsoft.com/windows/nanoserver'
            containerImage=$(readContainerImage "mcr.microsoft.com/windows/windowstestimage")
            When call updateMultiArchVersions "$containerImage"
            The variable MULTI_ARCH_VERSIONS[@] should be undefined
        End
    End

    Describe 'getLatestPkgVersionFromK8sVersion'
    COMPONENTS_FILEPATH="spec/parts/linux/cloud-init/artifacts/test_components.json"

        It 'returns correct latestVersion for Ubuntu'
            k8sVersion="1.32.3"
            OS="UBUNTU"
            OS_VERSION="22.04"
            When call getLatestPkgVersionFromK8sVersion "$k8sVersion" "fake-azure-acr-credential-provider" "$OS" "$OS_VERSION"
            The output should equal "1.32.3-ubuntu22.04u4"
        End
        It 'returns correct latestVersion for AzureLinux'
            k8sVersion="1.32.3"
            OS="AZURELINUX"
            OS_VERSION="3.0"
            When call getLatestPkgVersionFromK8sVersion "$k8sVersion" "fake-azure-acr-credential-provider" "$OS" "$OS_VERSION"
            The output should equal '1.32.3-4.azl3'
        End
        It 'returns highest latestVersion for Ubuntu if no matching k8s version'
            k8sVersion="1.34.0"
            OS="UBUNTU"
            OS_VERSION="22.04"
            When call getLatestPkgVersionFromK8sVersion "$k8sVersion" "fake-azure-acr-credential-provider" "$OS" "$OS_VERSION"
            The output should equal "1.32.3-ubuntu22.04u4"
        End
        It 'returns highest latestVersion for AzureLinux if no matching k8s version'
            k8sVersion="1.34.0"
            OS="AZURELINUX"
            OS_VERSION="3.0"
            When call getLatestPkgVersionFromK8sVersion "$k8sVersion" "fake-azure-acr-credential-provider" "$OS" "$OS_VERSION"
            The output should equal '1.32.3-4.azl3'
        End
    End

    Describe 'addKubeletNodeLabel'
        It 'should perform a no-op when the specified label already exists within the label string'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/agentpool=wp0"
            When call addKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The stdout should include 'kubelet node label kubernetes.azure.com/kubelet-serving-ca=cluster is already present, nothing to add'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should append the label when it does not already exist within the label string'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
            When call addKubeletNodeLabel "kubernetes.azure.com/kubelet-serving-ca=cluster"
            The stdout should not include 'kubelet node label kubernetes.azure.com/kubelet-serving-ca=cluster is already present, nothing to add'
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster'
        End
    End

    Describe 'removeKubeletNodeLabel'
        It 'should remove the specified label when it exists within kubelet node labels'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/agentpool=wp0"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should remove the specified label when it is the first label within kubelet node labels'
            KUBELET_NODE_LABELS="kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should remove the specified label when it is the last label within kubelet node labels'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should not alter kubelet node labels if the target label does not exist'
            KUBELET_NODE_LABELS="kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal 'kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0'
        End

        It 'should return an empty string if the only label within kubelet node labels is the target'
            KUBELET_NODE_LABELS="kubernetes.azure.com/kubelet-serving-ca=cluster"
            When call removeKubeletNodeLabel kubernetes.azure.com/kubelet-serving-ca=cluster
            The variable KUBELET_NODE_LABELS should equal ''
        End
    End

    Describe 'assert_refresh_token'
        # Helper function to create a mock JWT token
        # Usage: create_mock_jwt_token '{"permissions":{"actions":["read","pull"]}}'
        create_mock_jwt_token() {
            local payload="$1"
            # JWT format: header.payload.signature
            # We only care about the payload for this test
            local header='{"alg":"none","typ":"JWT"}'
            local encoded_header=$(printf '%s' "$header" | base64 -w0 | tr '+/' '-_' | tr -d '=')
            local encoded_payload=$(printf '%s' "$payload" | base64 -w0 | tr '+/' '-_' | tr -d '=')
            local signature="mock_signature"
            printf '%s.%s.%s' "$encoded_header" "$encoded_payload" "$signature"
        }

        It 'should fail for no read RBAC token'
            # Create a token with permissions but without "read"
            local token=$(create_mock_jwt_token '{"permissions":{"actions":["delete"]}}')
            When run assert_refresh_token "$token" "read"
            The status should equal 212  # ERR_ORAS_PULL_UNAUTHORIZED
            The stdout should include "Required action 'read' not found in token permissions"
        End

        It 'should pass read RBAC token'
            # Create a token with permissions including "read"
            local token=$(create_mock_jwt_token '{"permissions":{"actions":["read", "delete"]}}')
            When run assert_refresh_token "$token" "read"
            The status should be success
            The stdout should include "Token validation passed"
        End

        It 'should pass ABAC token'
            # Create a token without permissions field (ABAC token)
            local token=$(create_mock_jwt_token '{"sub":"test@example.com","exp":1234567890}')
            When run assert_refresh_token "$token" "read"
            The status should be success
            The stdout should include "No permissions field found in token. Assuming ABAC token, skipping permission validation"
        End
    End

    Describe 'oras_login_with_kubelet_identity'
        It 'should return if client_id or tenant_id is empty'
            local acr_url="unneeded.azurecr.io"
            local client_id=""
            local tenant_id=""
            When run oras_login_with_kubelet_identity $acr_url $client_id $tenant_id
            The status should be success
            The stdout should include "client_id or tenant_id are not set. Oras login is not possible, proceeding with anonymous pull"
        End
        It 'should fail if access token is an error'
            retrycmd_can_oras_ls_acr_anonymously() {
                return 1
            }
            retrycmd_get_aad_access_token(){
                echo "failed to retrieve kubelet identity token from IMDS, http code: 400, msg: {\"error\":\"invalid_request\",\"error_description\":\"Identity not found\"}"
                return $ERR_ORAS_PULL_UNAUTHORIZED
            }

            local acr_url="unneeded.azurecr.io"
            local client_id="failureClient"
            local tenant_id="mytenantID"
            When run oras_login_with_kubelet_identity $acr_url $client_id $tenant_id
            The status should be failure
            The stdout should include "failed to retrieve kubelet identity token"
        End
        It 'should fail if refresh token is an error'
            retrycmd_can_oras_ls_acr_anonymously() {
                return 1
            }
            retrycmd_get_aad_access_token(){
                echo "{\"access_token\":\"myAccessToken\"}"
            }
            retrycmd_get_refresh_token_for_oras(){
                echo "{\"error\":\"invalid_request\",\"error_description\":\"Identity not found\"}"
            }
            local acr_url="unneeded.azurecr.io"
            local client_id="myclientID"
            local tenant_id="failureID"
            When run oras_login_with_kubelet_identity $acr_url $client_id $tenant_id
            The status should be failure
            The stdout should include "failed to retrieve refresh token"
        End
        It 'should fail if oras cannot login'
            retrycmd_can_oras_ls_acr_anonymously() {
                return 1
            }
            retrycmd_get_aad_access_token(){
                echo "{\"access_token\":\"myAccessToken\"}"
            }
            retrycmd_get_refresh_token_for_oras(){
                echo "{\"refresh_token\":\"myRefreshToken\"}"
            }
            retrycmd_oras_login(){
                return 1
            }
            local acr_url="failed.azurecr.io"
            local client_id="myclientID"
            local tenant_id="mytenantID"
            When run oras_login_with_kubelet_identity $acr_url $client_id $tenant_id
            The status should be failure
            The stdout should include "failed to login to acr '$acr_url' with identity token"
        End
        It 'should succeed if oras can login'
            retrycmd_get_aad_access_token(){
                echo "{\"access_token\":\"myAccessToken\"}"
            }
            retrycmd_get_refresh_token_for_oras(){
                echo "{\"refresh_token\":\"myRefreshToken\"}"
            }
            retrycmd_oras_login(){
                return 0
            }
            retrycmd_can_oras_ls_acr_anonymously() {
                return 1
            }

            local acr_url="success.azurecr.io"
            local client_id="myclientID"
            local tenant_id="mytenantID"
            When run oras_login_with_kubelet_identity $acr_url $client_id $tenant_id
            The status should be success
            The stdout should include "successfully logged in to acr '$acr_url' with identity token"
            The stderr should be present
        End
    End

    Describe 'updateKubeBinaryRegistryURL'
        logs_to_events() {
            echo "mock logs to events calling with $1"
        }
        K8S_REGISTRY_REPO="oss/binaries/kubernetes"
        BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="mcr.microsoft.com"
        CPU_ARCH="amd64"
        It 'returns KUBE_BINARY_URL if it is already registry url'
            KUBE_BINARY_URL="mcr.microsoft.com/oss/binaries/kubernetes/kubernetes-node:v1.30.0-linux-amd64"

            When call updateKubeBinaryRegistryURL
            The variable KUBE_BINARY_REGISTRY_URL should equal "$KUBE_BINARY_URL"
            The output line 1 should equal "KUBE_BINARY_URL is a registry url, will use it to pull the kube binary"
        End
        It 'returns expected output from KUBE_BINARY_URL'
            KUBE_BINARY_URL="https://packages.aks.azure.com/kubernetes/v1.30.0-hotfix20241209/binaries/kubernetes-nodes-linux-amd64.tar.gz"
            KUBERNETES_VERSION="1.30.0"

            When call updateKubeBinaryRegistryURL
            The variable KUBE_BINARY_REGISTRY_URL should equal "$BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER/oss/binaries/kubernetes/kubernetes-node:v1.30.0-hotfix20241209-linux-amd64"
            The output line 1 should equal "Extracted version: v1.30.0-hotfix20241209 from KUBE_BINARY_URL: $KUBE_BINARY_URL"
        End
        It 'returns expected output for moonckae acs-mirror'
            KUBE_BINARY_URL="https://acs-mirror.azureedge.cn/kubernetes/v1.30.0-alpha/binaries/kubernetes-nodes-linux-amd64.tar.gz"
            KUBERNETES_VERSION="1.30.0"

            When call updateKubeBinaryRegistryURL
            The variable KUBE_BINARY_REGISTRY_URL should equal "$BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER/oss/binaries/kubernetes/kubernetes-node:v1.30.0-alpha-linux-amd64"
            The output line 1 should equal "Extracted version: v1.30.0-alpha from KUBE_BINARY_URL: $KUBE_BINARY_URL"
        End
        It 'uses KUBENETES_VERSION if KUBE_BINARY_URL is invalid'
            KUBE_BINARY_URL="https://invalidpath/v1.30.0-lts100/binaries/kubernetes-nodes-linux-amd64.tar.gz"
            KUBERNETES_VERSION="1.30.0"

            When call updateKubeBinaryRegistryURL
            The variable KUBE_BINARY_REGISTRY_URL should equal "$BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER/oss/binaries/kubernetes/kubernetes-node:v1.30.0-linux-amd64"
            The output line 1 should equal "KUBE_BINARY_URL is formatted unexpectedly, will use the kubernetes version as binary version: v$KUBERNETES_VERSION"
        End
    End

    Describe 'configureSSHService'
        systemctl() {
            case "$1" in
                "is-active")
                    if  [ "$3" = "ssh" ]; then
                        [ "${MOCK_SSH_SERVICE_ACTIVE:-false}" = "true" ] && return 0 || return 1
                    elif [ "$3" = "ssh.socket" ]; then
                        [ "${MOCK_SSH_SOCKET_ACTIVE:-false}" = "true" ] && return 0 || return 1
                    fi
                    ;;
                "is-enabled")
                    if  [ "$3" = "ssh.service" ]; then
                        [ "${MOCK_SSH_SERVICE_ENABLED:-false}" = "true" ] && return 0 || return 1
                    elif [ "$3" = "ssh.socket" ]; then
                        [ "${MOCK_SSH_SOCKET_ENABLED:-false}" = "true" ] && return 0 || return 1
                    fi
                    ;;
                "disable")
                    echo "systemctl disable called with: $*"
                    return 0
                    ;;
                *) return 0 ;;
            esac
        }

        systemctlEnableAndStart() {
            echo "systemctlEnableAndStart called with: $1"
            return 0
        }

        rm() {
            echo "rm called with: $1"
        }

        semverCompare() {
            # return 1 if MOCK_VERSION_COMPARE is 1, else return 0
            if [ "$MOCK_VERSION_COMPARE" = "1" ]; then
                return 1
            fi
            return 0
        }

        It 'handles non-Ubuntu OS correctly'
            When call configureSSHService "MARINER"
            The status should be success
        End

        It 'handles Ubuntu versions before 22.10 correctly'
            MOCK_VERSION_COMPARE=0
            When call configureSSHService "UBUNTU" "22.04"
            The status should be success
        End

        It 'handles Ubuntu 24.04 when service is already enabled'
            MOCK_VERSION_COMPARE=1
            MOCK_SSH_SERVICE_ENABLED="true"
            MOCK_SSH_SERVICE_ACTIVE="true"
            When call configureSSHService "UBUNTU" "24.04"
            The stdout should include "SSH service successfully reconfigured and started"
            The status should be success
        End

        It 'properly configures SSH for Ubuntu 24.04 with active socket'
            MOCK_VERSION_COMPARE=1
            MOCK_SSH_SOCKET_ACTIVE="true"
            MOCK_SSH_SERVICE_ENABLED="false"
            MOCK_SSH_SERVICE_ACTIVE="true"
            When call configureSSHService "UBUNTU" "24.04"
            The stdout should include "systemctlEnableAndStart called with: ssh"
            The status should be success
        End

        It 'returns error when SSH service fails to start'
            MOCK_VERSION_COMPARE=1
            MOCK_SSH_SOCKET_ACTIVE="true"
            MOCK_SSH_SERVICE_ENABLED="false"
            MOCK_SSH_SERVICE_ACTIVE="false"
            When call configureSSHService "UBUNTU" "24.04"
            The stdout should include "systemctlEnableAndStart called with: ssh"
            The status should equal $ERR_SYSTEMCTL_START_FAIL
        End
    End

    Describe 'isRegistryUrl'
        It 'returns true for valid registry url with tag'
            When call isRegistryUrl 'mcr.microsoft.com/component/binary:1.0'
            The status should be success
            The stdout should eq ''
            The stderr should eq ''
        End

        It 'returns false for url without tag'
            When call isRegistryUrl 'mcr.microsoft.com/component/binary'
            The status should be failure
            The stdout should eq ''
            The stderr should eq ''
        End

        It 'returns false for http url'
            When call isRegistryUrl 'https://example.com/file.tar.gz'
            The status should be failure
            The stdout should eq ''
            The stderr should eq ''
        End

        It 'returns true for registry url with complex tag'
            When call isRegistryUrl 'myrepo.azurecr.io/myimage:1.2.3-beta_4'
            The status should be success
            The stdout should eq ''
            The stderr should eq ''
        End

        It 'returns false for empty string'
            When call isRegistryUrl ''
            The status should be failure
            The stdout should eq ''
            The stderr should eq ''
        End
    End

    Describe 'extract_value_from_kubelet_flags'
        It 'extracts value for existing flag'
            KUBELET_FLAGS="--flag1=value1 --flag2=value2 --flag3=value3"
            When call extract_value_from_kubelet_flags "$KUBELET_FLAGS" "flag2"
            The output should eq "value2"
            The status should be success
        End

        It 'extracts value for existing flag with dash'
            KUBELET_FLAGS="--flag1=value1 --flag2=value2 --flag3=value3"
            When call extract_value_from_kubelet_flags "$KUBELET_FLAGS" "--flag1"
            The output should eq "value1"
            The status should be success
        End

        It 'returns empty string for non-existing flag'
            KUBELET_FLAGS="--flag1=value1 --flag2=value2 --flag3=value3"
            When call extract_value_from_kubelet_flags "$KUBELET_FLAGS" "flag4"
            The output should eq ""
            The status should be success
        End

        It 'handles flags without values'
            KUBELET_FLAGS="--flag1 --flag2=value2 --flag3"
            When call extract_value_from_kubelet_flags "$KUBELET_FLAGS" "flag1"
            The output should eq ""
            The status should be success
        End

        It 'handles empty KUBELET_FLAGS'
            KUBELET_FLAGS=""
            When call extract_value_from_kubelet_flags "$KUBELET_FLAGS" "flag1"
            The output should eq ""
            The status should be success
        End

        It 'handles flags with special characters in values'
            KUBELET_FLAGS="--flag1=value-with-dash --flag2=value_with_underscore --flag3=value.with.dot"
            When call extract_value_from_kubelet_flags "$KUBELET_FLAGS" "flag1"
            The output should eq "value-with-dash"
            The status should be success
        End
    End

    Describe 'get_sandbox_image'
        It 'get result from containerd config'
            get_sandbox_image_from_containerd_config(){
                echo "sandbox_image_from_containerd_config"
            }
            When call get_sandbox_image
            The output should eq "sandbox_image_from_containerd_config"
            The status should be success
        End

        It 'get result from kubelet flags if failed to get from containerd config'
            get_sandbox_image_from_containerd_config(){
                echo ""
            }
            extract_value_from_kubelet_flags(){
                echo "sandbox_image_from_kubelet_flags"
            }
            When call get_sandbox_image
            The output should eq "sandbox_image_from_kubelet_flags"
            The status should be success
        End

        It 'returns empty string if both failed'
            get_sandbox_image_from_containerd_config(){
                echo ""
            }
            extract_value_from_kubelet_flags(){
                echo ""
            }
            When call get_sandbox_image
            The output should eq ""
            The status should be success
        End
    End

    Describe 'get_sandbox_image_from_containerd_config'
        It 'returns empty string if config file does not exist'
            When call get_sandbox_image_from_containerd_config "non_existing_file"
            The output should eq ""
            The status should be success
        End

        It 'get result from containerd config'
            cat > existing_file << EOF
version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "sandbox_image_from_containerd_config"
[metrics]
  address = "0.0.0.0:10257"
EOF
            When call get_sandbox_image_from_containerd_config "existing_file"
            The output should eq "sandbox_image_from_containerd_config"
            The status should be success
			rm -f existing_file
        End
    End
End
