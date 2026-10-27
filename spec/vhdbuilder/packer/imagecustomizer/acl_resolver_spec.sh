#!/usr/bin/env shellspec

Describe 'ACL resolver handoff to ImageCustomizer'
  Parameters
    vhdbuilder/packer/vhd-image-builder-acl.json
    vhdbuilder/packer/vhd-image-builder-acl-arm64.json
  End

  setup() {
    TEST_ROOT="$(mktemp -d)"
    COMMAND_LOG="$TEST_ROOT/commands"
    mkdir -p "$TEST_ROOT/bin" "$TEST_ROOT/etc"
    : > "$COMMAND_LOG"
    /bin/ln -s /run/systemd/resolve/stub-resolv.conf "$TEST_ROOT/etc/resolv.conf"
    cat > "$TEST_ROOT/bin/sudo" <<'EOF'
#!/bin/bash
printf 'sudo %s\n' "$1" >> "$COMMAND_LOG"
# A second sudo could fail after waagent removes the Packer user.
[[ ! -e "$TEST_ROOT/sudo-called" ]] || exit 1
touch "$TEST_ROOT/sudo-called"
exec "$@"
EOF
    cat > "$TEST_ROOT/bin/waagent" <<'EOF'
#!/bin/bash
printf 'waagent %s\n' "$*" >> "$COMMAND_LOG"
[[ "${FAIL_DEPROVISION:-0}" == 0 ]] || exit 1
rm -f "$TEST_ROOT/etc/resolv.conf"
EOF
    cat > "$TEST_ROOT/bin/ln" <<'EOF'
#!/bin/bash
printf 'ln %s\n' "$*" >> "$COMMAND_LOG"
[[ "${FAIL_LINK:-0}" == 0 ]] || exit 1
[[ "$1" == -sfn && "$2" == /run/systemd/resolve/stub-resolv.conf && "$3" == /etc/resolv.conf ]] || exit 1
exec /bin/ln -sfn "$2" "$TEST_ROOT/etc/resolv.conf"
EOF
    cat > "$TEST_ROOT/bin/sync" <<'EOF'
#!/bin/bash
printf 'sync\n' >> "$COMMAND_LOG"
EOF
    chmod +x "$TEST_ROOT/bin/"*
    export TEST_ROOT COMMAND_LOG
    export PATH="$TEST_ROOT/bin:$PATH"
    unset FAIL_DEPROVISION FAIL_LINK
  }
  cleanup() {
    rm -rf "$TEST_ROOT"
  }
  deprovision() {
    # Execute the real template command, replacing only the Packer variable and
    # absolute waagent executable so no host deprovisioning is possible.
    jq -r --arg flag "$2" --arg waagent "$TEST_ROOT/bin/waagent" '
      .provisioners[] | select(.type == "shell") | .inline[]? |
      select(contains("/usr/bin/waagent -force -deprovision+user")) |
      gsub("/usr/bin/waagent"; $waagent) |
      gsub("\\{\\{user `acl_usr_overlays`\\}\\}"; $flag)
    ' "$1" > "$TEST_ROOT/deprovision.sh"
    bash "$TEST_ROOT/deprovision.sh" || return "$?"
    if [ -L "$TEST_ROOT/etc/resolv.conf" ]; then
      readlink "$TEST_ROOT/etc/resolv.conf"
    else
      echo absent
    fi
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'defaults overlay-specific cleanup off when the pipeline does not opt in'
    When call jq -r '.variables.acl_usr_overlays' "$1"
    The status should be success
    # shellcheck disable=SC2016
    The output should equal '{{or (env `ACL_USR_OVERLAYS`) `false`}}'
  End

  It 'restores the runtime symlink after deprovisioning without a second sudo'
    When call deprovision "$1" true
    The status should be success
    The output should equal '/run/systemd/resolve/stub-resolv.conf'
    The contents of file "$COMMAND_LOG" should include 'waagent -force -deprovision+user'
    The contents of file "$COMMAND_LOG" should include 'ln -sfn /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf'
    The contents of file "$COMMAND_LOG" should include 'sync'
  End

  It 'accepts the Azure boolean spelling True'
    When call deprovision "$1" True
    The status should be success
    The output should equal '/run/systemd/resolve/stub-resolv.conf'
  End

  It 'preserves deprovisioning behavior when overlays are disabled, including CVM'
    When call deprovision "$1" false
    The status should be success
    The output should equal absent
    The contents of file "$COMMAND_LOG" should not include 'ln -sfn'
    The contents of file "$COMMAND_LOG" should include 'sync'
  End

  It 'does not restore the resolver or sync if deprovisioning fails'
    export FAIL_DEPROVISION=1
    When call deprovision "$1" true
    The status should equal 125
    The contents of file "$COMMAND_LOG" should not include 'ln -sfn'
    The contents of file "$COMMAND_LOG" should not include 'sync'
  End

  It 'does not capture successfully if restoring the resolver fails'
    export FAIL_LINK=1
    When call deprovision "$1" true
    The status should equal 125
    The contents of file "$COMMAND_LOG" should include 'ln -sfn'
    The contents of file "$COMMAND_LOG" should not include 'sync'
  End
End
