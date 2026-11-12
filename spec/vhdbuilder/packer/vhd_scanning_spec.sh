#!/bin/bash
# shellcheck disable=SC2016,SC2034,SC2286,SC2317,SC2329

Describe 'VHD scanning CIS opt-in'
  scan_transition() (
    set -eu
    SCRIPT_NAME=vhd_scanning
    capture_benchmark() { echo "benchmark $*"; }
    process_benchmarks() { echo "benchmarks processed"; }
    trap 'echo "resource group cleanup"' EXIT

    if [ "$1" = unset ]; then
      unset PERFORM_CIS_SCAN
    else
      PERFORM_CIS_SCAN="$1"
    fi

    # Exercise the real default and the transition from Trivy to CIS without Azure calls.
    eval "$(sed -n '/^PERFORM_CIS_SCAN=/p' vhdbuilder/packer/vhd-scanning.sh)"
    eval "$(sed -n '/^echo -e "Trivy Scan Script Completed/,/^capture_benchmark "${SCRIPT_NAME}_cis_report_start"/p' vhdbuilder/packer/vhd-scanning.sh)"
    echo "CIS scanning reached"
  )

  Describe 'disabled scanning'
    Parameters
      unset
      ''
      false
      TRUE
      invalid
    End

    It "skips all CIS work for PERFORM_CIS_SCAN=$1 and finalizes the scan"
      When run scan_transition "$1"
      The status should be success
      The output should include 'Trivy Scan Script Completed'
      The output should include 'Skipping CIS assessment as PERFORM_CIS_SCAN is not set to true'
      The output should include 'benchmark vhd_scanning_overall true'
      The output should include 'benchmarks processed'
      The output should include 'resource group cleanup'
      The output should not include 'benchmark vhd_scanning_cis_report_start'
      The output should not include 'CIS scanning reached'
    End
  End

  It 'continues into existing CIS checks only when explicitly enabled'
    When run scan_transition true
    The status should be success
    The output should include 'Trivy Scan Script Completed'
    The output should include 'benchmark vhd_scanning_cis_report_start'
    The output should include 'CIS scanning reached'
    The output should not include 'Skipping CIS assessment'
  End
End
