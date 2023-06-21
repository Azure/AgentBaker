#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 60/364: 'display_login_attempts'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then

if grep -q -P "#.*pam_lastlog\.so" "/etc/pam.d/login"; then
    sed -i --follow-symlinks -E -e "s/^#(.*pam_lastlog\.so.*)$/\\1/" "/etc/pam.d/login" 
fi

if ! grep -q -P "^[[:space:]]*session[[:space:]]+[[:alnum:]]+[[:space:]]+pam_lastlog.so" "/etc/pam.d/login"; then
    sed -i --follow-symlinks '\`^# End /etc/pam.d/login.*`i session   required     pam_lastlog.so`' "/etc/pam.d/login"
fi



if [ -e "/etc/pam.d/login" ] ; then
    valueRegex="" defaultValue=""
    # non-empty values need to be preceded by an equals sign
    [ -n "${valueRegex}" ] && valueRegex="=${valueRegex}"
    # add an equals sign to non-empty values
    [ -n "${defaultValue}" ] && defaultValue="=${defaultValue}"

    # fix 'type' if it's wrong
    if grep -q -P "^\\s*(?"'!'"session\\s)[[:alnum:]]+\\s+[[:alnum:]]+\\s+pam_lastlog.so" < "/etc/pam.d/login" ; then
        sed --follow-symlinks -i -E -e "s/^(\\s*)[[:alnum:]]+(\\s+[[:alnum:]]+\\s+pam_lastlog.so)/\\1session\\2/" "/etc/pam.d/login"
    fi

    # fix 'control' if it's wrong
    if grep -q -P "^\\s*session\\s+(?"'!'"required)[[:alnum:]]+\\s+pam_lastlog.so" < "/etc/pam.d/login" ; then
        sed --follow-symlinks -i -E -e "s/^(\\s*session\\s+)[[:alnum:]]+(\\s+pam_lastlog.so)/\\1required\\2/" "/etc/pam.d/login"
    fi

    # fix the value for 'option' if one exists but does not match 'valueRegex'
    if grep -q -P "^\\s*session\\s+required\\s+pam_lastlog.so(\\s.+)?\\s+showfailed(?"'!'"${valueRegex}(\\s|\$))" < "/etc/pam.d/login" ; then
        sed --follow-symlinks -i -E -e "s/^(\\s*session\\s+required\\s+pam_lastlog.so(\\s.+)?\\s)showfailed=[^[:space:]]*/\\1showfailed${defaultValue}/" "/etc/pam.d/login"

    # add 'option=default' if option is not set
    elif grep -q -E "^\\s*session\\s+required\\s+pam_lastlog.so" < "/etc/pam.d/login" &&
            grep    -E "^\\s*session\\s+required\\s+pam_lastlog.so" < "/etc/pam.d/login" | grep -q -E -v "\\sshowfailed(=|\\s|\$)" ; then

        sed --follow-symlinks -i -E -e "s/^(\\s*session\\s+required\\s+pam_lastlog.so[^\\n]*)/\\1 showfailed${defaultValue}/" "/etc/pam.d/login"
    # add a new entry if none exists
    elif ! grep -q -P "^\\s*session\\s+required\\s+pam_lastlog.so(\\s.+)?\\s+showfailed${valueRegex}(\\s|\$)" < "/etc/pam.d/login" ; then
        echo "session required pam_lastlog.so showfailed${defaultValue}" >> "/etc/pam.d/login"
    fi
else
    echo "/etc/pam.d/login doesn't exist" >&2
fi

# remove 'silent' option
sed -i --follow-symlinks -E -e 's/^([^#]+pam_lastlog\.so[^#]*)\ssilent/\1/' '/etc/pam.d/login'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
