#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 108/364: 'accounts_logon_fail_delay'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q shadow-utils; then



# Set variables
var_accounts_fail_delay="4"



# Test if the config_file is a symbolic link. If so, use --follow-symlinks with sed.
# Otherwise, regular sed command will do.
sed_command=('sed' '-i')
if test -L "/etc/login.defs"; then
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
stripped_key=$(sed 's/[\^=\$,;+]*//g' <<< "^FAIL_DELAY")

# shellcheck disable=SC2059
printf -v formatted_output "%s %s" "$stripped_key" "$var_accounts_fail_delay"

# If the key exists, change it. Otherwise, add it to the config_file.
# We search for the key string followed by a word boundary (matched by \>),
# so if we search for 'setting', 'setting2' won't match.
if LC_ALL=C grep -q -m 1 -i -e "^FAIL_DELAY\\>" "/etc/login.defs"; then
    "${sed_command[@]}" "s/^FAIL_DELAY\\>.*/$formatted_output/gi" "/etc/login.defs"
else
    # \n is precaution for case where file ends without trailing newline
    printf '\n# Per %s: Set %s in %s\n' "$cce" "$formatted_output" "/etc/login.defs" >> "/etc/login.defs"
    printf '%s\n' "$formatted_output" >> "/etc/login.defs"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
