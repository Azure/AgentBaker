#!/usr/bin/env shellspec

Describe 'CSE completion markers'
    CSE_START_PATH="parts/linux/cloud-init/artifacts/cse_start.sh"

    markers_precede_failure_reporting() {
        awk '
            /touch \/opt\/azure\/containers\/base_prep\.complete/ { base_marker = NR }
            /touch \/opt\/azure\/containers\/provision\.complete/ { provision_marker = NR }
            /\[ "\$EXIT_CODE" -ne 0 \]/ { failure_check = NR }
            /exit "\$EXIT_CODE"/ { final_exit = NR }
            END {
                exit !(base_marker < failure_check &&
                       provision_marker < failure_check &&
                       failure_check < final_exit)
            }
        ' "$CSE_START_PATH"
    }

    It 'writes the stage marker before reporting the original exit code'
        When call markers_precede_failure_reporting

        The status should be success
    End
End
