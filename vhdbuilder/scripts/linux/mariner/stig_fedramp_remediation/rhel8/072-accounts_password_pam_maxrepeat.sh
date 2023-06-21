#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 72/364: 'accounts_password_pam_maxrepeat'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then


var_password_pam_maxrepeat="3"



pam_file="/etc/pam.d/system-password"
if grep -qE '^\s*password\s+requisite\s+pam_cracklib\.so' "$pam_file"; then
    # Replace 'pam_cracklib.so' (deprecated in v1.4.0) with 'pam_pwquality.so'
    sed -i --follow-symlinks 's/^\(\s*password\s\+requisite\s\+\)pam_cracklib\.so\(.*\)/\1pam_pwquality.so\2/' "$pam_file"
fi

# Test if the config_file is a symbolic link. If so, use --follow-symlinks with sed.
# Otherwise, regular sed command will do.
sed_command=('sed' '-i')
if test -L "/etc/security/pwquality.conf"; then
    sed_command+=('--follow-symlinks')
fi

# If the cce arg is empty, CCE is not assigned.
if [ -z "" ]; then
    cce="CCE"
else
    cce=""
fi

# Strip any search characters in the key arg so that the key can be replaced without
# adding any search characters to the config file.
stripped_key=$(sed 's/[\^=\$,;+]*//g' <<< "^maxrepeat")

# shellcheck disable=SC2059
printf -v formatted_output "%s = %s" "$stripped_key" "$var_password_pam_maxrepeat"

# If the key exists, change it. Otherwise, add it to the config_file.
# We search for the key string followed by a word boundary (matched by \>),
# so if we search for 'setting', 'setting2' won't match.
if LC_ALL=C grep -q -m 1 -i -e "^maxrepeat\\>" "/etc/security/pwquality.conf"; then
    "${sed_command[@]}" "s/^maxrepeat\\>.*/$formatted_output/gi" "/etc/security/pwquality.conf"
else
    # \n is precaution for case where file ends without trailing newline
    printf '\n# Per %s: Set %s in %s\n' "$cce" "$formatted_output" "/etc/security/pwquality.conf" >> "/etc/security/pwquality.conf"
    printf '%s\n' "$formatted_output" >> "/etc/security/pwquality.conf"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
