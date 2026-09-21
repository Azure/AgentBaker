#!/bin/bash

Describe 'format-mount-nvme-root.sh'
    script='parts/linux/cloud-init/artifacts/ubuntu/gb/format-mount-nvme-root.sh'

    It 'restores restrictive kubelet directory permissions after the bind mount'
        When run grep -F -e 'chown root:root "${KUBELET_DIR}"' -e 'chmod 0755 "${KUBELET_DIR}"' "${script}"
        The status should be success
        The output should include 'chown root:root "${KUBELET_DIR}"'
        The output should include 'chmod 0755 "${KUBELET_DIR}"'
    End

    It 'does not make the kubelet directory world-writable'
        When run grep -F 'chmod a+w' "${script}"
        The status should be failure
        The output should equal ''
    End
End
