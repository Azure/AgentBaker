#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 256/364: 'file_groupowner_var_log'")


chgrp 0 /var/log/
