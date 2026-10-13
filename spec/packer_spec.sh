#!/bin/bash

Describe 'Trident ACL agent startup'
  Include ./vhdbuilder/packer/packer_source.sh

  setup_agent() {
    MOCK_SERVICE_STATUS=0
    MOCK_ENABLE_STATUS=0
    systemctl() {
      [ "$*" = 'cat trident-acl-agent.service' ] || return 1
      return "$MOCK_SERVICE_STATUS"
    }
    systemctlEnableAndStart() {
      printf '%s\n' "$*"
      return "$MOCK_ENABLE_STATUS"
    }
  }
  BeforeEach 'setup_agent'

  It 'enables the kubeconfig trigger when the base contains the agent'
    When call enableTridentACLAgent
    The status should be success
    The output should equal 'trident-acl-agent.path 30'
  End

  It 'preserves compatibility with base images without the agent'
    MOCK_SERVICE_STATUS=1
    When call enableTridentACLAgent
    The status should be success
    The output should include 'skipping startup trigger'
    The output should not include 'trident-acl-agent.path 30'
  End

  It 'propagates failure to enable an installed agent'
    MOCK_ENABLE_STATUS=1
    When call enableTridentACLAgent
    The status should be failure
    The output should equal 'trident-acl-agent.path 30'
  End
End

Describe 'build-cosi-upload'
  setup() {
    export MOCK_GO_VERSION=1.27
    export MOCK_SYSTEM_CRYPTO=1
    export MOCK_BUILD_STATUS=0

    go() {
      case "$1 $2" in
        'env GOEXPERIMENT')
          [ "$MOCK_GO_VERSION" = 1.26 ]
          ;;
        'build -o')
          printf 'GOEXPERIMENT=%s CGO_ENABLED=%s\n' "${GOEXPERIMENT-}" "${CGO_ENABLED-}"
          return "$MOCK_BUILD_STATUS"
          ;;
        'version -m')
          printf '\tbuild\tmicrosoft_systemcrypto=%s\n' "$MOCK_SYSTEM_CRYPTO"
          ;;
        *) return 1 ;;
      esac
    }
    export -f go
  }
  BeforeEach 'setup'

  It 'uses the OpenSSL experiment on Go 1.26'
    MOCK_GO_VERSION=1.26
    When run make --no-print-directory -f packer.mk SHELL=/bin/bash build-cosi-upload
    The status should be success
    The output should include 'GOEXPERIMENT=ms_nocgo_opensslcrypto CGO_ENABLED=0'
  End

  It 'uses default system crypto on Go 1.27'
    When run make --no-print-directory -f packer.mk SHELL=/bin/bash build-cosi-upload
    The status should be success
    The output should include 'GOEXPERIMENT= CGO_ENABLED=0'
  End

  It 'rejects a binary without system crypto'
    MOCK_SYSTEM_CRYPTO=0
    When run make --no-print-directory -f packer.mk SHELL=/bin/bash build-cosi-upload
    The status should be failure
    The output should include 'Building cosi-upload binary'
    The error should include 'cosi-upload must be built with Microsoft Go system crypto'
  End

  It 'propagates a compiler failure'
    MOCK_BUILD_STATUS=17
    When run make --no-print-directory -f packer.mk SHELL=/bin/bash build-cosi-upload
    The status should be failure
    The output should include 'Building cosi-upload binary'
    The error should include 'Error 17'
  End
End