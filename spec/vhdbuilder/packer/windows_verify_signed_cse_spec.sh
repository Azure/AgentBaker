#!/bin/bash

Describe 'windows_verify_signed_cse.sh'
  VERIFY_SCRIPT="./.pipelines/scripts/windows_verify_signed_cse.sh"
  COMMIT="0123456789abcdef0123456789abcdef01234567"

  # Appends a signature block in the format the signing service writes.
  sign() {
    printf '\r\n# SIG # Begin signature block\r\n# MIIoKgYJKoZIhvcNAQcCoIIoGzCCKBcCAQExDzANBglghkgBZQMEAgEFADB5Bgor\r\n# BgEEAYI3AgEEoGswaTA0BgorBgEEAYI3AgEeMCYCAwEAAAQQH8w7YFlLCE63JNLG\r\n# KX7zUQIBAAIBAAIBAAIBAAIBADAxMA0GCWCGSAFlAwQCAQUABCA=\r\n# SIG # End signature block\r\n' >>"$1"
  }

  setup() {
    WORK_DIR=$(mktemp -d)
    UNSIGNED_DIR="$WORK_DIR/unsigned"
    SIGNED_DIR="$WORK_DIR/signed"
    PROVENANCE="$WORK_DIR/provenance.json"
    mkdir -p "$UNSIGNED_DIR/provisioningscripts"
    printf 'Write-Host "all"\n' >"$UNSIGNED_DIR/all.ps1"
    printf 'Write-Host "kubelet"\r\n' >"$UNSIGNED_DIR/provisioningscripts/kubeletstart.ps1"
    printf '[plugins]\n' >"$UNSIGNED_DIR/containerdtemplate.toml"
    cp -r "$UNSIGNED_DIR" "$SIGNED_DIR"
    sign "$SIGNED_DIR/all.ps1"
    sign "$SIGNED_DIR/provisioningscripts/kubeletstart.ps1"
    printf '{"sourceVersion": "%s"}\n' "$COMMIT" >"$PROVENANCE"
  }

  cleanup() {
    rm -rf "$WORK_DIR"
  }

  verify() {
    bash "$VERIFY_SCRIPT" "$UNSIGNED_DIR" "$SIGNED_DIR" "$PROVENANCE" "$COMMIT"
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'accepts scripts that only gained a signature block'
    When call verify
    The status should be success
    The output should include "Signed CSE scripts match commit $COMMIT"
  End

  It 'accepts a script whose own code mentions the signature marker'
    printf 'Write-Host "# SIG # Begin signature block"\n' >"$UNSIGNED_DIR/marker.ps1"
    cp "$UNSIGNED_DIR/marker.ps1" "$SIGNED_DIR/marker.ps1"
    sign "$SIGNED_DIR/marker.ps1"
    When call verify
    The status should be success
    The output should include "Signed CSE scripts match commit $COMMIT"
  End

  It 'rejects scripts signed from another commit'
    printf '{"sourceVersion": "%s"}\n' "fedcba9876543210fedcba9876543210fedcba98" >"$PROVENANCE"
    When call verify
    The status should be failure
    The output should include "signed scripts are from commit 'fedcba9876543210fedcba9876543210fedcba98'"
  End

  It 'rejects a script without a signature block'
    cp "$UNSIGNED_DIR/all.ps1" "$SIGNED_DIR/all.ps1"
    When call verify
    The status should be failure
    The output should include "all.ps1 has no signature block"
  End

  It 'rejects a script changed outside its signature block'
    printf 'Write-Host "changed"\n' >"$SIGNED_DIR/all.ps1"
    sign "$SIGNED_DIR/all.ps1"
    When call verify
    The status should be failure
    The output should include "all.ps1 differs from this commit's script outside its signature block"
  End

  It 'rejects code inside a signature block'
    printf 'Write-Host "all"\r\n# SIG # Begin signature block\r\nInvoke-Expression "evil"\r\n# SIG # End signature block\r\n' >"$SIGNED_DIR/all.ps1"
    When call verify
    The status should be failure
    The output should include "all.ps1 has a malformed signature block"
  End

  It 'rejects code after a signature block'
    printf 'Invoke-Expression "evil"\r\n' >>"$SIGNED_DIR/all.ps1"
    When call verify
    The status should be failure
    The output should include "all.ps1 has a malformed signature block"
  End

  It 'rejects a changed file that is not a PowerShell script'
    printf '[changed]\n' >"$SIGNED_DIR/containerdtemplate.toml"
    When call verify
    The status should be failure
    The output should include "containerdtemplate.toml differs from this commit's file"
  End

  It 'rejects added and missing files'
    rm "$SIGNED_DIR/provisioningscripts/kubeletstart.ps1"
    printf 'Write-Host "extra"\n' >"$SIGNED_DIR/extra.ps1"
    When call verify
    The status should be failure
    The output should include "the signed scripts contain different files than this commit's package"
    The output should include "./extra.ps1"
    The output should include "./provisioningscripts/kubeletstart.ps1"
  End
End
