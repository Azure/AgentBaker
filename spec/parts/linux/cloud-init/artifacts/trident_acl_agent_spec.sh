#!/bin/bash
# shellcheck disable=SC2016,SC2034

Describe 'Trident ACL Agent Node preflight'
  run_preflight() (
    checks=0
    hostname() { printf '%s\n' test-node; }
    sleep() { :; }
    kubectl() {
      checks=$((checks + 1))
      case "$1" in
        --kubeconfig) ;;
        *) return 2 ;;
      esac
      case "$availability" in
        registered) return 0 ;;
        delayed) [ "$checks" -gt 3 ] ;;
        interrupted) [ "$checks" -ne 2 ] ;;
        absent) return 1 ;;
      esac
    }
    script="$(sed -n '/^ExecStartPre=\/usr\/bin\/sh -c /,$p' \
      parts/linux/cloud-init/artifacts/acl/trident-acl-agent-override.conf |
      sed '1d; $s/'\''$//; s/\\$//; s/\$\$/\$/g; s|/opt/bin/kubectl|kubectl|g')"
    eval "$script"
  )

  It 'requires two successful checks for an existing Node'
    availability=registered
    When run run_preflight
    The status should be success
    The output should eq 'trident-acl-agent preflight: Node test-node registered after 2 check(s)'
  End

  It 'waits for delayed registration'
    availability=delayed
    When run run_preflight
    The status should be success
    The output should eq 'trident-acl-agent preflight: Node test-node registered after 5 check(s)'
  End

  It 'requires consecutive successes after an interrupted check'
    availability=interrupted
    When run run_preflight
    The status should be success
    The output should eq 'trident-acl-agent preflight: Node test-node registered after 4 check(s)'
  End

  It 'fails instead of starting the agent when the Node remains unavailable'
    availability=absent
    When run run_preflight
    The status should be failure
    The stderr should eq 'trident-acl-agent preflight: Node test-node not consistently available after 60 checks'
  End
End
