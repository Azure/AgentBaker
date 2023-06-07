#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 66/364: 'accounts_passwords_pam_faillock_unlock_time'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then


var_accounts_passwords_pam_faillock_unlock_time="0"




AUTH_FILES=("/etc/pam.d/system-auth")


for pam_file in "${AUTH_FILES[@]}"
do
    # Set the control for pam_unix.so correctly
    control="\[success=1 default=bad\]"
    if grep -q -P "^\\s*auth\\s+(?"'!'"${control})[][= [:alnum:]]+\\s+pam_unix\.so" < "$pam_file" ; then
        sed --follow-symlinks -i -E -e "s/^(\\s*auth\\s+)[][\= [:alnum:]]+(\\s+pam_unix\.so)/\\1${control}\\2/" "$pam_file"
    fi
# is auth required pam_faillock.so preauth present?
    if grep -qE '^\s*auth\s+required\s+pam_faillock\.so\s+preauth.*$' "$pam_file" ; then
    # is the option set?
        if grep -qE '^\s*auth\s+required\s+pam_faillock\.so\s+preauth.*'"unlock_time"'=([0-9]*).*$' "$pam_file" ; then
        # just change the value of option to a correct value
            sed -i --follow-symlinks 's/\(^auth.*required.*pam_faillock.so.*preauth.*silent.*\)\('"unlock_time"' *= *\).*/\1\2'"$var_accounts_passwords_pam_faillock_unlock_time"'/' "$pam_file"
    # the option is not set.
    else
        # append the option
            sed -i --follow-symlinks '/^auth.*required.*pam_faillock.so.*preauth.*silent.*/ s/$/ '"unlock_time"'='"$var_accounts_passwords_pam_faillock_unlock_time"'/' "$pam_file"
    fi
# auth required pam_faillock.so preauth is not present, insert the whole line
else
        control="\[success=1 default=bad\]"
        sed -i --follow-symlinks "/^auth.*$control.*pam_unix.so.*/i auth        required      pam_faillock.so preauth silent ""unlock_time"'='"$var_accounts_passwords_pam_faillock_unlock_time" "$pam_file"
fi
# is auth default pam_faillock.so authfail present?
    if grep -qE '^\s*auth\s+(\[default=die\])\s+pam_faillock\.so\s+authfail.*$' "$pam_file" ; then
    # is the option set?
        if grep -qE '^\s*auth\s+(\[default=die\])\s+pam_faillock\.so\s+authfail.*'"unlock_time"'=([0-9]*).*$' "$pam_file" ; then
        # just change the value of option to a correct value
            sed -i --follow-symlinks 's/\(^auth.*[default=die].*pam_faillock.so.*authfail.*\)\('"unlock_time"' *= *\).*/\1\2'"$var_accounts_passwords_pam_faillock_unlock_time"'/' "$pam_file"
    # the option is not set.
    else
        # append the option
            sed -i --follow-symlinks '/^auth.*[default=die].*pam_faillock.so.*authfail.*/ s/$/ '"unlock_time"'='"$var_accounts_passwords_pam_faillock_unlock_time"'/' "$pam_file"
    fi
# auth default pam_faillock.so authfail is not present, insert the whole line
else
        control="\[success=1 default=bad\]"
        sed -i --follow-symlinks "/^auth.*$control.*pam_unix.so.*/a auth        [default=die] pam_faillock.so authfail ""unlock_time"'='"$var_accounts_passwords_pam_faillock_unlock_time" "$pam_file"
fi
    
done

if ! grep -qE '^\s*account\s+required\s+pam_faillock\.so.*$' "/etc/pam.d/system-account" ; then
    sed -E -i --follow-symlinks '/^\s*account\s*required\s*pam_unix.so/i account     required      pam_faillock.so' "/etc/pam.d/system-account"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
