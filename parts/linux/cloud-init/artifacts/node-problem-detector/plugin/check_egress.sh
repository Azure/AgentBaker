#!/bin/bash

# If all endpoints are reachable, it returns OK.
#
# If any setup or dependencies are not available, it returns OK*.
# Note *, we do this to ensure we only event when we *know* there's an egress problem.
#
# If any endpoint is not reachable, it returns NOTOK and stdout message containing blocked endpoints / FQDN.
# Exit codes
readonly OK=0
readonly NONOK=1

if [ -f "/var/run/outbound-check-skipped" ]; then
    echo "Found file /var/run/outbound-check-skipped; skipping check egress test."
    exit $OK
elif [ -f "/opt/azure/outbound-check-skipped" ]; then
    echo "Found file /opt/azure/outbound-check-skipped; skipping check egress test."
    exit $OK
fi

# Global timeout for curl requests (for endpoints)
CURL_TIMEOUT_SECONDS=5

# Retry configuration (used for both connectivity and IMDS queries)
RETRY_COUNT=3       # number of attempts
RETRY_DELAY=2       # seconds to wait between attempts

# -----------------------------------------------------------------------------
# Parse optional argument(s)
# -----------------------------------------------------------------------------
SKIP_IMDS=0
if [[ "$1" == "--skip-imds" ]]; then
  SKIP_IMDS=1
  shift
fi

# -----------------------------------------------------------------------------
# Define endpoint arrays.
# -----------------------------------------------------------------------------
MCR_URL=()              # mcr and cdn
AAD_URL=()              # Entra authentication
RESOURCE_MANAGER_URL=() # k8s against Azure API
PACKAGES_URL=()         # package updates for apt (ubuntu) / tndf (mariner)
KUBE_BINARY_URL=()      # binaries such as kubelet / cni 
OS_PATCHES_URL=()       # other OS patches

AKS_COMMON_URLS=()      # common URLs for all AKS clusters
AKS_CLUSTER_URLS=()     # apiserver healthz, requires kubeconfig

AKS_KUBECONFIG_PATH=/var/lib/kubelet/kubeconfig
AKS_CA_CERT_PATH=/etc/kubernetes/certs/ca.crt
AKS_CERT_PATH=/etc/kubernetes/certs/client.crt
AKS_KEY_PATH=/etc/kubernetes/certs/client.key

# -----------------------------------------------------------------------------
# Function: curl_with_retry
#
# This function executes a curl command against a given URL with retry logic.
# By default it performs a HEAD request (using the -I flag) to quickly check
# connectivity. To perform a GET request (for example, when retrieving data),
# pass the special flag "--get" as the first argument after the URL.
#
# This function uses a timeout value determined by the environment variable
# CUSTOM_TIMEOUT if set, otherwise it defaults to CURL_TIMEOUT_SECONDS.
#
# Usage:
#   For HEAD requests (default):
#       curl_with_retry "http://example.com"
#
#   For GET requests:
#       curl_with_retry "http://example.com" --get -H "Some: header"
#
# On success, the function echoes the output and returns 0.
# On failure, it echoes the final error output and returns 1.
# -----------------------------------------------------------------------------
curl_with_retry() {
  local url="$1"
  shift
  local method="HEAD"
  # Check if the caller requested a GET request.
  if [[ "$1" == "--get" ]]; then
      method="GET"
      shift
  fi

  # Use CUSTOM_TIMEOUT if set, otherwise use CURL_TIMEOUT_SECONDS.
  local timeout="${CUSTOM_TIMEOUT:-$CURL_TIMEOUT_SECONDS}"

  local attempt=1
  local output
  while [ $attempt -le $RETRY_COUNT ]; do
    if [ "$method" == "HEAD" ]; then
      # For HEAD requests, discard stdout so only errors are captured.
      output=$(curl -S -s -m ${timeout} "$@" -I "$url" 2>&1 >/dev/null)
    else
      # For GET requests, capture the full output.
      output=$(curl -S -s -m ${timeout} "$@" "$url" 2>&1)
    fi

    if [ $? -eq 0 ]; then
      echo "$output"
      return 0
    fi
    attempt=$((attempt+1))
    sleep $RETRY_DELAY
  done
  echo "$output"
  return 1
}

# -----------------------------------------------------------------------------
# Get Azure environment info.
#
# If the --skip-imds argument is provided, skip querying IMDS and use a default
# environment (AZURECLOUD). Otherwise, attempt to fetch the environment info
# from IMDS (using GET) with retries and a timeout of 2 seconds. If IMDS cannot
# be contacted or parsed, exit gracefully with OK.
# -----------------------------------------------------------------------------
if [[ $SKIP_IMDS -eq 1 ]]; then
  echo "Skipping IMDS check, using default Azure environment (AZURECLOUD)."
  az_environment="AZURECLOUD"
else
  if command -v jq &> /dev/null; then
    imds_url="http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01&format=json"
    # Set a custom timeout of 2 seconds for IMDS.
    CUSTOM_TIMEOUT=2
    imds_response=$(curl_with_retry "$imds_url" --get --noproxy "*" -H Metadata:true)
    unset CUSTOM_TIMEOUT
    if [ $? -eq 0 ]; then
      az_environment=$(echo "$imds_response" | jq -r '.azEnvironment')
    else
      echo "Could not contact IMDS endpoint after ${RETRY_COUNT} attempts. Exiting gracefully."
      exit $OK
    fi

    # If no environment was parsed, exit gracefully.
    if [ -z "$az_environment" ]; then
      echo "Could not parse IMDS response. Exiting gracefully."
      exit $OK
    fi

    # Convert environment to uppercase for case matching.
    az_environment=$(echo "$az_environment" | tr '[:lower:]' '[:upper:]')
  else
    echo "Missing expected tool jq. Exiting gracefully."
    exit $OK
  fi
fi

# -----------------------------------------------------------------------------
# Populate endpoint arrays based on the Azure environment.
# -----------------------------------------------------------------------------
case "$az_environment" in
  "AZUREGERMANCLOUD")
    MCR_URL+=("https://mcr.microsoft.com")
    AAD_URL+=("https://login.microsoftonline.de")
    RESOURCE_MANAGER_URL+=("https://management.microsoftazure.de")
    PACKAGES_URL+=("https://packages.microsoft.com")
    KUBE_BINARY_URL+=("https://acs-mirror.azureedge.net/acs-mirror/healthz" "https://packages.aks.azure.com/acs-mirror/healthz")
    ;;
  "AZURECHINACLOUD")
    MCR_URL+=("https://mcr.azk8s.cn")
    AAD_URL+=("https://login.chinacloudapi.cn")
    RESOURCE_MANAGER_URL+=("https://management.chinacloudapi.cn")
    PACKAGES_URL+=("https://packages.microsoft.com")
    KUBE_BINARY_URL+=("https://mirror.azk8s.cn")
    ;;
  "AZUREUSGOVERNMENT" | "AZUREUSGOVERNMENTCLOUD")
    MCR_URL+=("https://mcr.microsoft.com")
    AAD_URL+=("https://login.microsoftonline.us")
    RESOURCE_MANAGER_URL+=("https://management.usgovcloudapi.net")
    PACKAGES_URL+=("https://packages.microsoft.com")
    KUBE_BINARY_URL+=("https://acs-mirror.azureedge.net/acs-mirror/healthz" "https://packages.aks.azure.com/acs-mirror/healthz")
    ;;
  "USNAT" | "USNATCLOUD")
    MCR_URL+=("https://mcr.microsoft.eaglex.ic.gov")
    AAD_URL+=("https://login.microsoftonline.eaglex.ic.gov")
    RESOURCE_MANAGER_URL+=("https://management.azure.eaglex.ic.gov")
    KUBE_BINARY_URL+=("https://aksteleportusnat.blob.core.eaglex.ic.gov")
    ;;
  "USSEC" | "USSECCLOUD")
    MCR_URL+=("https://mcr.microsoft.scloud")
    AAD_URL+=("https://login.microsoftonline.microsoft.scloud")
    RESOURCE_MANAGER_URL+=("https://management.azure.microsoft.scloud")
    KUBE_BINARY_URL+=("https://aksteleportussec2.blob.core.microsoft.scloud")
    ;;
  "AZURECLOUD" | "AZUREPUBLICCLOUD")
    # Azure public (and other unrecognized clouds default to public endpoints)
    MCR_URL+=("https://mcr.microsoft.com")
    AAD_URL+=("https://login.microsoftonline.com")
    RESOURCE_MANAGER_URL+=("https://management.azure.com")
    PACKAGES_URL+=("https://packages.microsoft.com")
    KUBE_BINARY_URL+=("https://acs-mirror.azureedge.net/acs-mirror/healthz" "https://packages.aks.azure.com/acs-mirror/healthz")
    ;;
  *)
    echo "Unknown Azure environment: '$az_environment', skip image and binary URL check."
    exit $OK
    ;;
esac

# -----------------------------------------------------------------------------
# If the required files for the apiserver check are present, extract the APISERVER
# FQDN and add the healthz URL.
# -----------------------------------------------------------------------------
if ! [ -f "$AKS_KUBECONFIG_PATH" ] || \
   ! [ -f "$AKS_CA_CERT_PATH" ] || \
   ! [ -f "$AKS_CERT_PATH" ] || \
   ! [ -f "$AKS_KEY_PATH" ]; then 
    echo "Kubeconfig file or cert files not found, skip apiserver check."
else
    APISERVER_FQDN=$(grep server "$AKS_KUBECONFIG_PATH" | awk -F"server: https://" '{print $2}' | cut -d : -f 1)
    
    # Check if FQDN contains "privatelink" and exit OK if it does
    if [[ "$APISERVER_FQDN" == *"privatelink"* ]]; then
        echo "Private cluster detected (FQDN contains 'privatelink'), skipping egress check."
        exit $OK
    fi    

    AKS_CLUSTER_URLS+=("https://${APISERVER_FQDN}/healthz")
fi

# -----------------------------------------------------------------------------
# Combine endpoints into a common list.
# -----------------------------------------------------------------------------
AKS_COMMON_URLS=("${MCR_URL[@]}" "${AAD_URL[@]}" "${RESOURCE_MANAGER_URL[@]}" "${PACKAGES_URL[@]}" "${KUBE_BINARY_URL[@]}" "${OS_PATCHES_URL[@]}")
if [[ ${#AKS_COMMON_URLS[@]} -eq 0 ]] && [[ ${#AKS_CLUSTER_URLS[@]} -eq 0 ]]; then
  echo "No endpoints are checked: could not determine APIServer and host info. This might be a transient issue. Please retry later."
  exit $OK
fi

# -----------------------------------------------------------------------------
# Variables to collect unreachable URLs.
# -----------------------------------------------------------------------------
UNREACHED_URLS=()
any_failed=0

append_unreached_url() {
  local url=$1
  local error_message=$2
  domain_endpoint=${url#*://}
  if [[ "$error_message" != *"$domain_endpoint"* ]]; then
    error_message="$error_message: $url"
  fi
  # Add a trailing space to preserve spaces in array elements.
  UNREACHED_URLS+=("${error_message} ")
  any_failed=1
}

# -----------------------------------------------------------------------------
# Check connectivity for common endpoints using curl_with_retry.
# (These use the default HEAD request.)
# -----------------------------------------------------------------------------
for url in "${AKS_COMMON_URLS[@]}"; do
  if ! output=$(curl_with_retry "$url"); then
    append_unreached_url "$url" "$output"
  fi
done

# -----------------------------------------------------------------------------
# Check connectivity for cluster endpoints (which require certificates)
# using curl_with_retry.
# -----------------------------------------------------------------------------
for url in "${AKS_CLUSTER_URLS[@]}"; do
  if ! output=$(curl_with_retry "$url" --cacert "$AKS_CA_CERT_PATH" --cert "$AKS_CERT_PATH" --key "$AKS_KEY_PATH"); then
    append_unreached_url "$url" "$output"
  fi
done

# -----------------------------------------------------------------------------
# Report and exit based on whether any endpoints failed.
# -----------------------------------------------------------------------------
if [[ $any_failed -ne 0 ]]; then
   echo "Required endpoints are unreachable ($(IFS=';'; echo "${UNREACHED_URLS[*]}")), aka.ms/AArpzy5 for more information."
   exit $NONOK
else
  # echo "Node has egress access to AKS basic endpoints/FQDNs."
  exit $OK
fi
