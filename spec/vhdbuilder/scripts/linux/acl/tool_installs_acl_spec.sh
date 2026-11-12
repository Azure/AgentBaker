#!/bin/bash
# shellcheck disable=SC2329

Describe 'enableTridentACLAgent'
  BeforeAll "eval \"\$(sed -n '/^enableTridentACLAgent()/,/^}/p' './parts/root/acl/opt/azure/image-build/tool_installs_distro.sh')\""

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
