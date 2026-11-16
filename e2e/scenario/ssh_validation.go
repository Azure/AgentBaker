package scenario

const validateSSHServiceDisabledScript = `#!/bin/bash
set -euo pipefail
. /etc/os-release
policy_dir="${SSH_POLICY_DIR:-/etc/aks/ssh-policy}"
unit_dir="${SSH_SYSTEMD_UNIT_DIR:-/etc/systemd/system}"
if [[ ! -f "$policy_dir/disabled" ]]; then
    echo "FAILED: persistent SSH disable marker is missing"
    exit 1
fi

if [[ "$ID" == "ubuntu" ]]; then
    units=(ssh.service)
    # A drop-in can exist even when the image does not ship the socket unit.
    if [[ "$(systemctl show ssh.socket --property=LoadState --value)" != not-found ]]; then
        units+=(ssh.socket)
    fi
elif [[ "$ID" == "azurecontainerlinux" || ( "$ID" == "azurelinux" && "${VARIANT_ID:-}" == "azurecontainerlinux" ) ]]; then
    units=(sshd.socket sshd.service)
else
    units=(sshd.service)
fi

for unit in "${units[@]}"; do
    load_state=$(systemctl show "$unit" --property=LoadState --value)
    if [[ "$load_state" != loaded && "$load_state" != masked ]]; then
        echo "FAILED: $unit is not loaded: $load_state"
        exit 1
    fi
    if ! grep -Fxq "ConditionPathExists=!$policy_dir/disabled" "$unit_dir/$unit.d/90-aks-ssh-access.conf"; then
        echo "FAILED: $unit has no persistent SSH disable condition"
        exit 1
    fi
    active_state=$(systemctl is-active "$unit" || true)
    enabled_state=$(systemctl is-enabled "$unit" || true)
    echo "$unit: active=$active_state enabled=$enabled_state"

    if [[ "$active_state" != "inactive" ]]; then
        echo "FAILED: $unit is not inactive"
        exit 1
    fi
    if [[ "$load_state" != masked ]]; then
        # A skipped condition returns success; verify that no listener was started.
        systemctl start "$unit"
        if [[ "$(systemctl show "$unit" --property=ConditionResult --value)" != no ]]; then
            echo "FAILED: $unit is not blocked by its persistent condition"
            exit 1
        fi
        if [[ "$(systemctl is-active "$unit" || true)" != inactive ]]; then
            echo "FAILED: $unit activated despite the SSH disable policy"
            exit 1
        fi
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
