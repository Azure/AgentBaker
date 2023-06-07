#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 330/364: 'no_host_based_files'")


find / -ignore_readdir_race -type f -name "shosts.equiv" -exec rm -f {} \; || true
