#!/bin/bash
# shellcheck disable=SC2016,SC2329

# ShellSpec tests for assertPackageVersion helper function.
# The function under test lives in vhdbuilder/packer/test/lib/package_helpers.sh.

# Source the shared helper (provides err + assertPackageVersion).
# shellcheck source=../../../../vhdbuilder/packer/test/lib/package_helpers.sh
. "./vhdbuilder/packer/test/lib/package_helpers.sh"

Describe 'assertPackageVersion helper function'

  Describe 'dpkg path (deb-based systems)'
    It 'succeeds when installed deb version matches expected version'
      command() { return 0; }
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
      command() { return 0; }
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
      command() { return 0; }
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
