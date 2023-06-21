#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule: 'custom_python_39_crypt_fips'")

# "Backport" this commit: https://github.com/python/cpython/commit/069fefdaf42490f1e00243614fb5f3d5d2614b81
# This issue only effects python 3.9, <= 3.8 does not include the buggy code, and >= 3.10 has the fix.
if [ -f /usr/lib/python3.9/crypt.py ]; then
    sed -i 's~if e.errno == errno.EINVAL:~if e.errno in {errno.EINVAL, errno.EPERM, errno.ENOSYS}:~g' /usr/lib/python3.9/crypt.py
fi