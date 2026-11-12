package scenario

const validateSSHServiceDisabledScript = `#!/bin/bash
set -euo pipefail
. /etc/os-release

if [[ "$ID" == "ubuntu" ]]; then
    units=(ssh.service)
    # Ubuntu 22.10+ can serve port 22 from ssh.socket, and an enabled-but-inactive
    # socket reopens it on the next boot, so check it whenever the image ships it.
    if systemctl cat ssh.socket &>/dev/null; then
        units+=(ssh.socket)
    fi
elif [[ "$ID" == "azurecontainerlinux" || ( "$ID" == "azurelinux" && "${VARIANT_ID:-}" == "azurecontainerlinux" ) ]]; then
    units=(sshd.socket sshd.service)
else
    units=(sshd.service)
fi

for unit in "${units[@]}"; do
    active_state=$(systemctl is-active "$unit" || true)
    enabled_state=$(systemctl is-enabled "$unit" || true)
    echo "$unit: active=$active_state enabled=$enabled_state"

    if [[ "$active_state" != "inactive" ]]; then
        echo "FAILED: $unit is not inactive"
        exit 1
    fi
    if [[ "$enabled_state" != "disabled" ]]; then
        echo "FAILED: $unit is not disabled"
        exit 1
    fi
done

listeners=$(ss -H -ltn 'sport = :22') || {
    echo "FAILED: unable to inspect TCP port 22"
    exit 1
}
if [[ -n "$listeners" ]]; then
    echo "FAILED: TCP port 22 is listening"
    exit 1
fi

echo "SUCCESS: SSH units are disabled and stopped"
`
