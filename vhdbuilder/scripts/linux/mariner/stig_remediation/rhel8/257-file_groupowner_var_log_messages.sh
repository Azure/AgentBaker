#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 257/364: 'file_groupowner_var_log_messages'")


chgrp 0 /var/log/messages
