#!/bin/bash
# This gets us the error codes we use and the os and such.
source /home/packer/provision_source.sh

assignRootPW() {
    set +x
    if grep '^root:[!*]:' /etc/shadow; then
        VERSION=$(grep DISTRIB_RELEASE /etc/*-release | cut -f 2 -d "=")
        SALT=$(openssl rand -base64 5)
        SECRET=$(openssl rand -base64 37)
        CMD="import crypt, getpass, pwd; print(crypt.crypt('$SECRET', '\$6\$$SALT\$'))"
        if [[ "${VERSION}" == "22.04" || "${VERSION}" == "24.04" ]]; then
            HASH=$(python3 -c "$CMD")
        else
            HASH=$(python -c "$CMD")
        fi

        echo 'root:'$HASH | /usr/sbin/chpasswd -e || exit $ERR_CIS_ASSIGN_ROOT_PW
    fi
    set -x
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

    if [[ -f /etc/default/grub ]]; then
        chmod 644 /etc/default/grub || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    fi

    if [[ -f /etc/crontab ]]; then
        chmod 0600 /etc/crontab || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    fi
    for filepath in /etc/cron.hourly /etc/cron.daily /etc/cron.weekly /etc/cron.monthly /etc/cron.d; do
        if [[ -e $filepath ]]; then
            chmod 0600 $filepath || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
        fi
    done

    # Docs: https://www.man7.org/linux/man-pages/man1/crontab.1.html
    # If cron.allow exists, then cron.deny is ignored. To minimize who can use cron, we
    # always want cron.allow and will default it to empty if it doesn't exist.
    # We also need to set appropriate permissions on it.
    # Since it will be ignored anyway, we delete cron.deny.
    touch /etc/cron.allow || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    chmod 640 /etc/cron.allow || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
    rm -rf /etc/cron.deny || exit $ERR_CIS_ASSIGN_FILE_PERMISSION
}

# Helper function to replace or append settings to a setting file.
# This abstracts the general logic of:
#   1. Search for the setting (via a pattern passed in).
#   2. If it's there, replace it with desired setting line; otherwise append it to the end of the file.
#   3. Validate that there is now exactly one instance of the setting, and that it is the one we want.
replaceOrAppendSetting() {
    local SEARCH_PATTERN=$1
    local SETTING_LINE=$2
    local FILE=$3

    # Search and replace/append.
    if grep -E "$SEARCH_PATTERN" "$FILE" >/dev/null; then
        sed -E -i "s|${SEARCH_PATTERN}|${SETTING_LINE}|g" "$FILE" || exit $ERR_CIS_APPLY_PASSWORD_CONFIG
    else
        echo -e "\n${SETTING_LINE}" >>"$FILE"
    fi

    # After replacement/append, there should be exactly one line that sets the setting,
    # and it must have the value we want.
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
#   '${1}='         -- Then the setting name followed by '='
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

# Creates the search pattern and setting lines for the core dump settings, and calls through
# to do the replacement. Note that this uses extended regular expressions, so both
# grep and sed need to be called as such.
#
# The search pattern is:
#  '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
#  '${1}='         -- Then the setting name followed by '='
#  '.*$'           -- Then 0 or nore of any character which is the end of the line.
#
# This is based on a combination of the syntax for the file (https://www.man7.org/linux/man-pages/man5/coredump.conf.5.html)
# and real examples we've found.
replaceOrAppendCoreDump() {
    replaceOrAppendSetting "^#{0,1} {0,1}${1}=.*$" "${1}=${2}" /etc/systemd/coredump.conf
}

configureCoreDump() {
    replaceOrAppendCoreDump Storage none
    replaceOrAppendCoreDump ProcessSizeMax 0
}

fixUmaskSettings() {
    # CIS requires the default UMASK for account creation to be set to 027, so change that in /etc/login.defs.
    replaceOrAppendLoginDefs UMASK 027

    # It also requires that nothing in etc/profile.d sets umask to anything less restrictive than that.
    # Mariner/AzureLinux sets umask directly in /etc/profile after sourcing everything in /etc/profile.d. But it also has /etc/profile.d/umask.sh
    # which sets umask (but is then ignored). We don't want to simply delete /etc/profile.d/umask.sh, because if we take an update to
    # the package that supplies it, it would just be copied over again.
    # This is complicated by an oddity/bug in the auditing script cis uses, which will flag line in a file with the work umask in the file name
    # that doesn't set umask correctly. So we can't just comment out all the lines or have any comments that explain what we're doing.
    # So since we can't delete the file, we just overwrite it with the correct umask setting. This duplicates what /etc/profile does, but
    # it does no harm and works with the tools.
    # Note that we use printf to avoid a trailing newline.
    local umask_sh="/etc/profile.d/umask.sh"
    if isMarinerOrAzureLinux "$OS"; then
        if [[ -f "${umask_sh}" ]]; then
            printf "umask 027" >${umask_sh}
        fi
    fi
}

function maskNfsServer() {
    # If nfs-server.service exists, we need to mask it per CIS requirement.
    # Note that on ubuntu systems, it isn't installed but on mariner/azurelinux we need it
    # due to a dependency, but disable it by default.
    if systemctl list-unit-files nfs-server.service >/dev/null; then
        systemctl --now mask nfs-server || $ERR_SYSTEMCTL_MASK_FAIL
    fi
}

function addFailLockDir() {
    # Mariner/AzureLinux uses pamd faillocking, which requires a directory to store the faillock files.
    # Default is /var/run/faillock, but that's a tmpfs, so we need to use /var/log/faillock instead.
    # But we need to leave settings alone for other skus.
    if isMarinerOrAzureLinux "$OS" ; then
        # Replace or append the dir setting in /etc/security/faillock.conf
        # Docs: https://www.man7.org/linux/man-pages/man5/faillock.conf.5.html
        #
        # Search pattern is:
        # '^#{0,1} {0,1}' -- Line starts with 0 or 1 '#' followed by 0 or 1 space
        # 'dir\s+'        -- Then the setting name followed by one or more whitespace characters
        # '.*$'           -- Then 0 or nore of any character which is the end of the line.
        #
        # This is based on a combination of the syntax for the file and real examples we've found.
        local fail_lock_dir="/var/log/faillock"
        mkdir -p ${fail_lock_dir}
        replaceOrAppendSetting "^#{0,1} {0,1}dir\s+.*$" "dir = ${fail_lock_dir}" /etc/security/faillock.conf
    fi
}

applyCIS() {
    setPWExpiration
    assignRootPW
    assignFilePermissions
    configureCoreDump
    fixUmaskSettings
    maskNfsServer
    addFailLockDir
}

applyCIS

#EOF
