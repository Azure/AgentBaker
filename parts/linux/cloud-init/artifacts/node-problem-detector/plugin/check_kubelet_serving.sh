#!/bin/bash

# This checks the state of the kubelet's serving certificate. Note that this brings in a dependency on openssl.
# A problem will be reported in the following cases:
# - kubelet does not have any valid serving certificate
# - kubelet's serving certificate is self-signed, if TOLERATE_SELF_SIGNED is false

OK=0
NOTOK=1
UNKNOWN=2
SELFSIGNED=$NOTOK
[ "${TOLERATE_SELF_SIGNED:-true}" == "true" ] && SELFSIGNED=$OK

CHECK_KUBELET_STATUS_TIMEOUT=10 # 10 second timeout for checking kubelet status
OPENSSL_TIMEOUT=30 # 30 second timeout for cert validation

CLUSTER_CA_FILE_PATH="/etc/kubernetes/certs/ca.crt"

if ! which openssl >/dev/null; then
  echo "openssl is not supported"
  exit $UNKNOWN
fi

if ! which systemctl >/dev/null; then
  echo "systemd is not supported"
  exit $UNKNOWN
fi

if ! timeout $CHECK_KUBELET_STATUS_TIMEOUT systemctl -q is-active kubelet || ! curl -m "$CHECK_KUBELET_STATUS_TIMEOUT" -f -s -S http://127.0.0.1:10248/healthz >/dev/null 2>&1; then
  echo "kubelet server is down, unable to check cert validity"
  exit $OK # we already check kubelet is up and running in a separate plugin, thus there's no need to fire in this case
fi

if [ ! -f "$CLUSTER_CA_FILE_PATH" ]; then
    echo "cannot find ca cert, unable to check cert validity"
    exit $UNKNOWN
fi

# verify return code 0 - CA-signed
# verify return code 18/19 - self-signed
# exits with code 124 if openssl times out
RESULT=$(timeout $OPENSSL_TIMEOUT openssl s_client -connect 127.0.0.1:10250 -CAfile $CLUSTER_CA_FILE_PATH -noservername -verify_return_error 2>&1 </dev/null)
if [ $? -eq 124 ]; then
  echo "openssl timed out, unable to check serving cert validity"
  exit $UNKNOWN
fi

if grep -i "tlsv1 alert internal error" <<< "$RESULT" >/dev/null 2>&1; then
  echo "kubelet does not seem to have any valid serving certificate"
  exit $NOTOK
fi

if grep -i "verify return code: 18" <<< "$RESULT" >/dev/null 2>&1 || grep -i "verify return code: 19" <<< "$RESULT" >/dev/null 2>&1; then
  echo "kubelet serving certificate is self-signed"
  exit $SELFSIGNED
fi

if grep -i "verify return code: 0" <<< "$RESULT" >/dev/null 2>&1; then
  echo "kubelet serving certificate is signed by the cluster CA"
  exit $OK
fi

echo "unknown result: $RESULT"
exit $UNKNOWN