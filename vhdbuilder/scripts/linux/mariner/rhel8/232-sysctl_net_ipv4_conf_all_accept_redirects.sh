#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 232/364: 'sysctl_net_ipv4_conf_all_accept_redirects'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then




if [[ ! -e '/etc/sysctl.conf' ]]; then
    ln -s /etc/sysctl.d/50-security-hardening.conf /etc/sysctl.conf
fi
sysctl_net_ipv4_conf_all_accept_redirects_value="0"



#
# Set runtime for net.ipv4.conf.all.accept_redirects
#

if [ ! -e /.buildenv ]; then
/sbin/sysctl -q -n -w net.ipv4.conf.all.accept_redirects="$sysctl_net_ipv4_conf_all_accept_redirects_value"
fi


#
# If net.ipv4.conf.all.accept_redirects present in /etc/sysctl.conf, change value to appropriate value
#	else, add "net.ipv4.conf.all.accept_redirects = value" to /etc/sysctl.conf
#
# Test if the config_file is a symbolic link. If so, use --follow-symlinks with sed.
# Otherwise, regular sed command will do.
sed_command=('sed' '-i')
if test -L "/etc/sysctl.conf"; then
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
stripped_key=$(sed 's/[\^=\$,;+]*//g' <<< "^net.ipv4.conf.all.accept_redirects")

# shellcheck disable=SC2059
printf -v formatted_output "%s = %s" "$stripped_key" "$sysctl_net_ipv4_conf_all_accept_redirects_value"

# If the key exists, change it. Otherwise, add it to the config_file.
# We search for the key string followed by a word boundary (matched by \>),
# so if we search for 'setting', 'setting2' won't match.
if LC_ALL=C grep -q -m 1 -i -e "^net.ipv4.conf.all.accept_redirects\\>" "/etc/sysctl.conf"; then
    "${sed_command[@]}" "s/^net.ipv4.conf.all.accept_redirects\\>.*/$formatted_output/gi" "/etc/sysctl.conf"
else
    # \n is precaution for case where file ends without trailing newline
    printf '\n# Per %s: Set %s in %s\n' "$cce" "$formatted_output" "/etc/sysctl.conf" >> "/etc/sysctl.conf"
    printf '%s\n' "$formatted_output" >> "/etc/sysctl.conf"
fi


#
# Some sysctl vars will not load until after their related kernel modules have loaded (network especially). Wait for
# the system to be fully booted then reload the values as a work around.
#

cat > /etc/systemd/system/reload_sysctl.service << EOF
[Unit]
Description=Reload sysctl values at end of boot
After=multi-user.target

[Install]
WantedBy=multi-user.target

[Service]
RemainAfterExit=yes
ExecStart=/sbin/sysctl -p
Type=oneshot
EOF

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" unmask 'reload_sysctl.service'
"$SYSTEMCTL_EXEC" start 'reload_sysctl.service'
"$SYSTEMCTL_EXEC" enable 'reload_sysctl.service'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
