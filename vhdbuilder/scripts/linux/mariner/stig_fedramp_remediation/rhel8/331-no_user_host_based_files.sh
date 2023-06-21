#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 331/364: 'no_user_host_based_files'")


find / -ignore_readdir_race -type f -name ".shosts" -exec rm -f {} \; || true
