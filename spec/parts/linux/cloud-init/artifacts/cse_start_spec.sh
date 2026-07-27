#!/usr/bin/env shellspec

Describe 'CSE completion markers'
    CSE_START_PATH="parts/linux/cloud-init/artifacts/cse_start.sh"

    It 'checks provisioning failure before both fail-closed marker writes'
        When run awk '
            /\[ "\$EXIT_CODE" -ne 0 \]/ { failure_check = NR }
            failure_check && !failure_exit && /exit "\$EXIT_CODE"/ { failure_exit = NR }
            /touch \/opt\/azure\/containers\/base_prep\.complete \|\| exit 1/ { base_marker = NR }
            /touch \/opt\/azure\/containers\/provision\.complete \|\| exit 1/ { provision_marker = NR }
            END {
                exit !(failure_check < failure_exit &&
                       failure_exit < base_marker &&
                       base_marker < provision_marker)
            }
        ' "$CSE_START_PATH"

        The status should be success
    End
End
