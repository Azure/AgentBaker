#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 221/364: 'network_configure_name_resolution'")

# Some common public DNS servers
# Each link will also support default DHCP DNS
#  Cloudflare, Cloudflare IPv6, Google, Google IPv6
ips="1.1.1.1 2606:4700:4700::1111 8.8.8.8 2001:4860:4860::8888"

# Test if the config_file is a symbolic link. If so, use --follow-symlinks with sed.
# Otherwise, regular sed command will do.
sed_command=('sed' '-i')
if test -L "/etc/resolv.conf"; then
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
stripped_key=$(sed 's/[\^=\$,;+]*//g' <<< "^nameserver")

# shellcheck disable=SC2059
printf -v formatted_output "%s %s" "$stripped_key" "127.0.0.53"

# If the key exists, change it. Otherwise, add it to the config_file.
# We search for the key string followed by a word boundary (matched by \>),
# so if we search for 'setting', 'setting2' won't match.
if LC_ALL=C grep -q -m 1 -i -e "^nameserver\\>" "/etc/resolv.conf"; then
    "${sed_command[@]}" "s/^nameserver\\>.*/$formatted_output/gi" "/etc/resolv.conf"
else
    # \n is precaution for case where file ends without trailing newline
    printf '\n# Per %s: Set %s in %s\n' "$cce" "$formatted_output" "/etc/resolv.conf" >> "/etc/resolv.conf"
    printf '%s\n' "$formatted_output" >> "/etc/resolv.conf"
fi

# Test if the config_file is a symbolic link. If so, use --follow-symlinks with sed.
# Otherwise, regular sed command will do.
sed_command=('sed' '-i')
if test -L "/etc/systemd/resolved.conf"; then
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
stripped_key=$(sed 's/[\^=\$,;+]*//g' <<< "^DNS")

# shellcheck disable=SC2059
printf -v formatted_output "%s=%s" "$stripped_key" "${ips}"

# If the key exists, change it. Otherwise, add it to the config_file.
# We search for the key string followed by a word boundary (matched by \>),
# so if we search for 'setting', 'setting2' won't match.
if LC_ALL=C grep -q -m 1 -i -e "^DNS\\>" "/etc/systemd/resolved.conf"; then
    "${sed_command[@]}" "s/^DNS\\>.*/$formatted_output/gi" "/etc/systemd/resolved.conf"
else
    # \n is precaution for case where file ends without trailing newline
    printf '\n# Per %s: Set %s in %s\n' "$cce" "$formatted_output" "/etc/systemd/resolved.conf" >> "/etc/systemd/resolved.conf"
    printf '%s\n' "$formatted_output" >> "/etc/systemd/resolved.conf"
fi
