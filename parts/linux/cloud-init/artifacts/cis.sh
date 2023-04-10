#!/bin/bash

assignRootPW() {
  if grep '^root:[!*]:' /etc/shadow; then
    VERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    SALT=$(openssl rand -base64 5)
    SECRET=$(openssl rand -base64 37)
    CMD="import crypt, getpass, pwd; print(crypt.crypt('$SECRET', '\$6\$$SALT\$'))"
    if [[ "${VERSION}" == "22.04" ]]; then
      HASH=$(python3 -c "$CMD")
    else
      HASH=$(python -c "$CMD")
    fi

    echo 'root:'$HASH | /usr/sbin/chpasswd -e || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
  fi
}

assignFilePermissions() {
    FILES="
    auth.log
    alternatives.log
    cloud-init.log
    cloud-init-output.log
    daemon.log
    dpkg.log
    kern.log
    lastlog
    waagent.log
    syslog
    unattended-upgrades/unattended-upgrades.log
    unattended-upgrades/unattended-upgrades-dpkg.log
    azure-vnet-ipam.log
    azure-vnet-telemetry.log
    azure-cnimonitor.log
    azure-vnet.log
    kv-driver.log
    blobfuse-driver.log
    blobfuse-flexvol-installer.log
    landscape/sysinfo.log
    "
    for FILE in ${FILES}; do
        FILEPATH="/var/log/${FILE}"
        DIR=$(dirname "${FILEPATH}")
        mkdir -p ${DIR} || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
        touch ${FILEPATH} || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
        chmod 640 ${FILEPATH} || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    done
    find /var/log -type f -perm '/o+r' -exec chmod 'g-wx,o-rwx' {} \;
    chmod 600 /etc/passwd- || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 600 /etc/shadow- || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 600 /etc/group- || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 600 /etc/gshadow- || exit $ERR_CIS_ASSIGN_FILE_PERMISSION

    if [[ -f /etc/default/grub ]]; then
        chmod 644 /etc/default/grub || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    fi

    if [[ -f /etc/crontab ]]; then
        chmod 0600 /etc/crontab || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    fi
    touch /etc/cron.allow || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chown root:root /etc/cron.allow || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 0640 /etc/cron.allow || exit $ERR_CIS_ASSIGN_FILE_PERMISSION

    if [[ -f /etc/cron.deny ]]; then
        chmod 0640 /etc/cron.deny || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    fi

    for filepath in /etc/cron.hourly /etc/cron.daily /etc/cron.weekly /etc/cron.monthly /etc/cron.d; do
      chmod 0600 $filepath || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    done
}

# Helper function to replace or append settings to a setting file.
# This abstracts the general logic of:
#   1. Search for the setting (via a pattern passed in).
#   2. If it's there, replace it with desired setting line.
#   3. Validate that there is now exactly instance of the setting, and that it is the one we want.
replaceOrAppendSetting() {
    local SEARCH_PATTERN=$1
    local SETTING_LINE=$2
    local FILE=$3

    # Search and replace/append.
    if grep -E "$SEARCH_PATTERN" "$FILE">/dev/null ; then
        sed -E -i "s|${SEARCH_PATTERN}|${SETTING_LINE}|g" "$FILE" || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
    else
        echo -e "\n${SETTING_LINE}" >> "$FILE"
    fi

    # At the end, there must be only line for the setting, and it must be what we want.
    # If not, then there's something wrong with this script.
    if [[ $(grep -E "$SEARCH_PATTERN" "$FILE") != "$SETTING_LINE" ]]; then
        echo "replacement was wrong"
        exit $ERR_CIS_APPLY_PASSWORD_CONFIG
    fi
}

# Creates the search pattern and setting lines for login.defs settings, and calls through
# to do the replacement. Note that this uses extended regular expressions, so both
# grep and sed need to be called as such.
#
# The search pattern is:
#   '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
#   '${1}\s+'       -- Then the setting name followed by one or more whitespace characters
#   '[0-9]+$'       -- Then one more more number, which is the setting value, which is the end of the line.
#
# This is based on a combination of the syntax for the file and real examples we've found.
replaceOrAppendLoginDefs() {
    replaceOrAppendSetting "^#{0,1} {0,1}${1}\s+[0-9]+$" "${1} ${2}" /etc/login.defs
}

# Creates the search pattern and setting lines for useradd default settings, and calls through
# to do the replacement. Note that this uses extended regular expressions, so both
# grep and sed need to be called as such.
#
# The search pattern is:
#   '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
#   '${1}='         -- Then the setting name followed by one or more whitespace characters
#   '.*$'           -- Then 0 or nore of any character which is the end of the line.
#                      Note that this allows for a setting value to be there or not.
#
# This is based on a combination of the syntax for the file and real examples we've found.
replaceOrAppendUserAdd() {
    replaceOrAppendSetting "^#{0,1} {0,1}${1}=.*$" "${1}=${2}" /etc/default/useradd
}

setPWExpiration() {
    replaceOrAppendLoginDefs PASS_MAX_DAYS 90
    replaceOrAppendLoginDefs PASS_MIN_DAYS 7
    replaceOrAppendUserAdd INACTIVE 30
}

removeUnneededFiles() {
    rm -f /etc/profile.d/umask.sh
    rm -f /etc/motd
    rm -f /etc/pam.d/common-password
    rm -f /etc/pam.d/common-auth

}

disableCoreDumps() {
  rm -f /etc/systemd/coredump.conf
  cat "Storage=none" >> /etc/systemd/coredump.conf
  cat "ProcessSizeMax=0" >> /etc/systemd/coredump.conf
}

replaceOrAppendChronyd() {
    replaceOrAppendSetting "^#{0,1} {0,1}${1}=.*$" "${1}=${2}" /etc/sysconfig/chronyd
}

fixChronyUser() {
  replaceOrAppendChronyd OPTIONS "-u chrony"
}

setPamdPasswordConf() {
  cat > /etc/pam.d/system-auth << "EOF"
# Begin /etc/pam.d/system-auth

auth      required    pam_unix.so

# Security patch for msid: 5.3.2
# Ensure lockout for failed password attempts is configured
auth      required    pam_faillock.so deny=5 unlock_time=900

password  required    pam_pwhistory.so  remember=5
password  sufficient  pam_unix.so       sha512 obscure use_authtok try_first_pass

# End /etc/pam.d/system-auth
EOF

  cat > /etc/pam.d/system-password << "EOF"
# Begin /etc/pam.d/system-password

# use sha512 hash for encryption, use shadow, and try to use any previously
# defined authentication token (chosen password) set by any prior module
password  requisite   pam_pwquality.so
password  required    pam_pwhistory.so  remember=5
password  required    pam_unix.so       sha512 shadow try_first_pass

# End /etc/pam.d/system-password
EOF
}

applyCIS() {
  setPWExpiration
  assignRootPW
  assignFilePermissions
  removeUnneededFiles
  replaceOrAppendLoginDefs UMASK 027
  disableCoreDumps
  mv /etc/profile.d/umask.sh /etc/profile.d/50-umask.sh
}

applyCIS

#EOF
