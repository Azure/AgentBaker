#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 81/364: 'disable_ctrlaltdel_burstaction'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q systemd; then

# Test if the config_file is a symbolic link. If so, use --follow-symlinks with sed.
# Otherwise, regular sed command will do.
sed_command=('sed' '-i')
if test -L "/etc/systemd/system.conf"; then
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
stripped_key=$(sed 's/[\^=\$,;+]*//g' <<< "^CtrlAltDelBurstAction=")

# shellcheck disable=SC2059
printf -v formatted_output "%s=%s" "$stripped_key" "none"

# If the key exists, change it. Otherwise, add it to the config_file.
# We search for the key string followed by a word boundary (matched by \>),
# so if we search for 'setting', 'setting2' won't match.
if LC_ALL=C grep -q -m 1 -i -e "^CtrlAltDelBurstAction=\\>" "/etc/systemd/system.conf"; then
    "${sed_command[@]}" "s/^CtrlAltDelBurstAction=\\>.*/$formatted_output/gi" "/etc/systemd/system.conf"
else
    # \n is precaution for case where file ends without trailing newline
    printf '\n# Per %s: Set %s in %s\n' "$cce" "$formatted_output" "/etc/systemd/system.conf" >> "/etc/systemd/system.conf"
    printf '%s\n' "$formatted_output" >> "/etc/systemd/system.conf"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
