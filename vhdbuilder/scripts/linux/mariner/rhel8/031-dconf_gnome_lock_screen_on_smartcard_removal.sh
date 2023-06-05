#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 31/364: 'dconf_gnome_lock_screen_on_smartcard_removal'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q gdm && { [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; }; then

# Check for setting in any of the DConf db directories
# If files contain ibus or distro, ignore them.
# The assignment assumes that individual filenames don't contain :
readarray -t SETTINGSFILES < <(grep -r "\\[org/gnome/settings-daemon/peripherals/smartcard\\]" "/etc/dconf/db/" | grep -v 'distro\|ibus' | cut -d":" -f1)
DCONFFILE="/etc/dconf/db/local.d/00-security-settings"
DBDIR="/etc/dconf/db/local.d"

mkdir -p "${DBDIR}"

if [ "${#SETTINGSFILES[@]}" -eq 0 ]
then
    [ ! -z ${DCONFFILE} ] || echo "" >> ${DCONFFILE}
    printf '%s\n' "[org/gnome/settings-daemon/peripherals/smartcard]" >> ${DCONFFILE}
    printf '%s=%s\n' "removal-action" "'lock-screen'" >> ${DCONFFILE}
else
    escaped_value="$(sed -e 's/\\/\\\\/g' <<< "'lock-screen'")"
    if grep -q "^\\s*removal-action\\s*=" "${SETTINGSFILES[@]}"
    then
        
        sed -i "s/\\s*removal-action\\s*=\\s*.*/removal-action=${escaped_value}/g" "${SETTINGSFILES[@]}"
    else
        sed -i "\\|\\[org/gnome/settings-daemon/peripherals/smartcard\\]|a\\removal-action=${escaped_value}" "${SETTINGSFILES[@]}"
    fi
fi

dconf update
# Check for setting in any of the DConf db directories
LOCKFILES=$(grep -r "^/org/gnome/settings-daemon/peripherals/smartcard/removal-action$" "/etc/dconf/db/" | grep -v 'distro\|ibus' | cut -d":" -f1)
LOCKSFOLDER="/etc/dconf/db/local.d/locks"

mkdir -p "${LOCKSFOLDER}"

if [[ -z "${LOCKFILES}" ]]
then
    echo "/org/gnome/settings-daemon/peripherals/smartcard/removal-action" >> "/etc/dconf/db/local.d/locks/00-security-settings-lock"
fi

dconf update

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
