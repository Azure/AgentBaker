#!/bin/bash

OK=0
NOTOK=1

PUBLIC_SETTINGS_PATH="/etc/node-problem-detector.d/public-settings.json"

EC=$OK

if [ -f $PUBLIC_SETTINGS_PATH ]; then
    ENABLE_UU=$(jq -r '."enable-uu"' $PUBLIC_SETTINGS_PATH)
    if [ "$ENABLE_UU" = "true" ]; then
        # only want to exit early if we find the file and enable-uu is true
        if [ -f /opt/azure/unattended-upgrade.re-enabled ]; then
          # skip if unattended upgrades is already re-enabled
          exit $EC
        fi
        # re-enabling what was disabled previously
        echo "Re-enabling unattended upgrades..."
        if grep 'Unattended-Upgrade "0"' /etc/apt/apt.conf.d/20auto-upgrades &> /dev/null; then
            # enable unattended upgrades in the default location
            sed -i 's/Update-Package-Lists "0"/Update-Package-Lists "1"/ ; s/Unattended-Upgrade "0"/Unattended-Upgrade "1"/' /etc/apt/apt.conf.d/20auto-upgrades
            touch /opt/azure/unattended-upgrade.re-enabled
            EC=$NOTOK
        fi
        if grep 'Unattended-Upgrade "0"' /etc/apt/apt.conf.d/99periodic &> /dev/null; then
            # enable unattended upgrades in the agentbaker uu config location
            sed -i 's/Update-Package-Lists "0"/Update-Package-Lists "1"/ ; s/Unattended-Upgrade "0"/Unattended-Upgrade "1"/' /etc/apt/apt.conf.d/99periodic
            touch /opt/azure/unattended-upgrade.re-enabled
            EC=$NOTOK
        fi
        exit $EC
    fi
fi

if [ -f /opt/azure/unattended-upgrade.disabled ]; then
    # skip if unattended upgrades is already disabled
    exit $EC
fi

if [ -f $PUBLIC_SETTINGS_PATH ]; then
    DISABLE_UU=$(jq -r '."disable-uu"' $PUBLIC_SETTINGS_PATH)
    if [ "$DISABLE_UU" = "false" ]; then
        exit $EC
    fi
fi

build_date="$(date -d "$(grep -oP 'VSTS Build NUMBER: \K[^.]+' /opt/azure/vhd-install.complete)" "+%s")"
if [ "${build_date}" -gt "$(date -d "20230901" "+%s")" ]; then
    # skip if node image build date is after 2023-09-01
    exit $EC
fi

if grep 'Unattended-Upgrade "1"' /etc/apt/apt.conf.d/20auto-upgrades &> /dev/null; then
    # disable unattended upgrades in the default location
    echo "Unattended Upgrade is enabled, disabling..."
    sed -i 's/Update-Package-Lists "1"/Update-Package-Lists "0"/ ; s/Unattended-Upgrade "1"/Unattended-Upgrade "0"/' /etc/apt/apt.conf.d/20auto-upgrades
    EC=$NOTOK
fi

if grep 'Unattended-Upgrade "1"' /etc/apt/apt.conf.d/99periodic &> /dev/null; then
    # disable unattended upgrades in the agentbaker uu config location
    echo "Unattended Upgrade is enabled, disabling..."
    sed -i 's/Update-Package-Lists "1"/Update-Package-Lists "0"/ ; s/Unattended-Upgrade "1"/Unattended-Upgrade "0"/' /etc/apt/apt.conf.d/99periodic
    EC=$NOTOK
fi

if dpkg -l | grep linux-azure | grep 6.2.0.1009 &> /dev/null; then
    # shellcheck disable=SC2046
    DEBIAN_FRONTEND=noninteractive apt -y purge $(dpkg -l | awk '$2 ~ /linux.*azure/ && $3 ~ /^6\.2\.0.1009/ { print $2; }') &> /dev/null
    EC=$NOTOK
fi

touch /opt/azure/unattended-upgrade.disabled

exit $EC
