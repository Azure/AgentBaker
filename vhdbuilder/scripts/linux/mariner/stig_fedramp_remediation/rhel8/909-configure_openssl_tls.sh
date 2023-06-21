#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 11/274: 'configure_openssl_tls'")

CONF_FILE="/etc/pki/tls/openssl.cnf"
correct_value='MinProtocol = TLSv1.2'

if ! grep -q "${correct_value}" ${CONF_FILE}; then
    # We need to get the existing value, using PCRE to maintain same regex
    existing_value=$(grep -Po 'MinProtocol' ${CONF_FILE} || true)

    if [[ ! -z ${existing_value} ]]; then
        # replace existing_value with correct_value
        sed -i "s/${existing_value}/${correct_value}/g" ${CONF_FILE}
    else
        # ***NOTE*** #
        # This probably means this file is not here or it's been modified
        # unintentionally.
        # ********** #
        # echo correct_value to end
        echo ${correct_value} >> ${CONF_FILE}
    fi
fi
