#!/bin/bash

# 5.4.4 Ensure default user umask is 027 or more restrictive
umask 027

# 5.4.3.2 Ensure default user shell timeout is configured
readonly TMOUT=900 ; export TMOUT
