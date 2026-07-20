#!/bin/bash
# shellcheck disable=SC2329

# ShellSpec tests for assertPackageVersion helper function in linux-vhd-content-test.sh.
# Covers: dpkg path, rpm path, package-not-installed path, version mismatch path, and epoch stripping.

Describe 'assertPackageVersion helper function'
  # Source the functions by writing extracted content to a temp file (avoids eval expansion issues with ${Status}/${Version}).
  BeforeAll '
    _tmpfunc=$(mktemp)
    sed -n "/^err()/,/^}/p" "./vhdbuilder/packer/test/linux-vhd-content-test.sh" > "$_tmpfunc"
    sed -n "/^assertPackageVersion()/,/^}/p" "./vhdbuilder/packer/test/linux-vhd-content-test.sh" >> "$_tmpfunc"
    . "$_tmpfunc"
    rm -f "$_tmpfunc"
  '

  Describe 'dpkg path (deb-based systems)'
    It 'succeeds when installed deb version matches expected version'
      # Mock dpkg-query to simulate moby-containerd 2.3.2-ubuntu24.04u2 installed
      dpkg-query() {
        case "$3" in
          '${Status}') echo "install ok installed" ;;
          '${Version}') echo "2.3.2-ubuntu24.04u2" ;;
        esac
      }
      When call assertPackageVersion "testContainerd" "moby-containerd" "2.3.2-ubuntu24.04u2"
      The status should equal 0
      The output should include "matches expected"
    End

    It 'fails when installed deb version does not match expected version'
      dpkg-query() {
        case "$3" in
          '${Status}') echo "install ok installed" ;;
          '${Version}') echo "2.3.2-ubuntu24.04u2" ;;
        esac
      }
      When call assertPackageVersion "testContainerd" "moby-containerd" "2.3.2-ubuntu24.04u1"
      The status should equal 1
      The error should include "does not match expected"
    End

    It 'strips epoch prefix from dpkg version before comparison'
      dpkg-query() {
        case "$3" in
          '${Status}') echo "install ok installed" ;;
          '${Version}') echo "1:1.7.33-ubuntu22.04u1" ;;
        esac
      }
      When call assertPackageVersion "testContainerd" "moby-containerd" "1.7.33-ubuntu22.04u1"
      The status should equal 0
    End
  End

  Describe 'rpm path (RPM-based systems)'
    It 'succeeds when installed rpm version matches expected version'
      # Mock: dpkg-query not available, rpm available
      dpkg-query() { return 1; }
      command() {
        case "$2" in
          dpkg-query) return 1 ;;
          rpm) return 0 ;;
        esac
      }
      rpm() {
        case "$1" in
          -q)
            if [ "$2" = "--queryformat" ]; then
              echo "2.2.4-4.azl3"
            else
              return 0
            fi
            ;;
        esac
      }
      When call assertPackageVersion "testContainerd" "containerd2" "2.2.4-4.azl3"
      The status should equal 0
    End

    It 'fails when installed rpm version does not match expected version'
      dpkg-query() { return 1; }
      command() {
        case "$2" in
          dpkg-query) return 1 ;;
          rpm) return 0 ;;
        esac
      }
      rpm() {
        case "$1" in
          -q)
            if [ "$2" = "--queryformat" ]; then
              echo "2.2.4-3.azl3"
            else
              return 0
            fi
            ;;
        esac
      }
      When call assertPackageVersion "testContainerd" "containerd2" "2.2.4-4.azl3"
      The status should equal 1
      The error should include "does not match expected"
    End
  End

  Describe 'package not installed path'
    It 'fails when package is not installed (neither dpkg nor rpm finds it)'
      dpkg-query() {
        case "$3" in
          '${Status}') echo "unknown ok not-installed" ;;
        esac
      }
      command() {
        case "$2" in
          dpkg-query) return 0 ;;
          rpm) return 1 ;;
        esac
      }
      When call assertPackageVersion "testContainerd" "moby-containerd" "2.3.2-ubuntu24.04u2"
      The status should equal 1
      The error should include "is not installed"
    End
  End
End
