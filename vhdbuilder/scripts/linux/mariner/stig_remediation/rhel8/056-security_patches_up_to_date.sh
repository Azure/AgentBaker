#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 56/364: 'security_patches_up_to_date'")


dnf -y update
