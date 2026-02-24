#! /bin/bash
#
# AKS Log Collector
#
# This script collects information and logs that are useful to AKS engineering
# for support and uploads them to the Azure host via a private API. These log
# bundles are available to engineering when customers open a support case and
# are especially useful for troubleshooting failures of networking or
# kubernetes daemons.
#
# This script runs via a systemd unit and slice that limits it to low CPU
# priority and 128MB RAM, to avoid impacting other system functions.

# Shell options - remove non-matching globs, don't care about case, and use
# extended pattern matching
shopt -s nullglob nocaseglob extglob

# Fetch configuration from IMDS - expected is a JSON object in the aks-log-collector tag
# JSON body:
# {
#   "disable": false,
#   "files": [ "/etc/skel/.bashrc", "/etc/skel/.bash_profile" ],
#   "pod_log_namespaces": [ "default", "pahealy" ],
#   "iptables": false,
#   "nftables": false,
#   "netns": true
# }
CONFIG=$(curl -s -H Metadata:true --noproxy '*' 'http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01' | jq '.tagsList[] | select(.name=="aks-log-collector") | .value | fromjson')

# If the JSON object has "disable": true, then quit.
<<<"$CONFIG" jq -esRr 'try fromjson catch null | .disable? // false' >/dev/null && {
  printf "IMDS tag aks-log-collector disable==true, quitting.\n"
  exit 0
}

COLLECT_IPTABLES=$(<<<"$CONFIG" jq -esRr 'try fromjson catch null | .iptables? // false')
COLLECT_NFTABLES=$(<<<"$CONFIG" jq -esRr 'try fromjson catch null | .nftables? // false')
COLLECT_NETNS=$(<<<"$CONFIG" jq -esRr 'try fromjson catch null | .netns? // false')

### START CONFIGURATION
ZIP="aks_logs.zip"

# Log bundle upload max size is limited to 100MB
ENFORCE_MAX_ZIP_SIZE="${ENFORCE_MAX_ZIP_SIZE:-true}"
MAX_SIZE=104857600

# File globs to include
# Smaller and more critical files are closer to the top so that we can be certain they're included.
declare -a GLOBS

# Add explicitly included files at the top to make sure they're included
for FILE in $(<<<"$CONFIG" jq -esRr 'try fromjson catch ("" | halt_error) | .files[]'); do
  GLOBS+=("${FILE}")
done

# Add extra_namespaces to the top to make sure those pod logs are included
for NAMESPACE in $(<<<"$CONFIG" jq -esRr 'try fromjson catch ("" | halt_error) | .pod_log_namespaces[]'); do
  GLOBS+=("/var/log/pods/${NAMESPACE}_*/**/*")
done

# AKS specific entries
GLOBS+=(/etc/default/kubelet)
GLOBS+=(/var/log/azure/*/*)
GLOBS+=(/var/log/azure/*/*/*)
GLOBS+=(/var/log/syslog*)
GLOBS+=(/var/log/journal*)

### END CONFIGURATION

command -v zip >/dev/null || {
  echo "Error: zip utility not found. Please install zip."
  exit 255
}

# Create a temporary directory to store results in
WORKDIR="$(mktemp -d)"
# check if tmp dir was created
if [ -z "$WORKDIR" ] || [ "$WORKDIR" = "/" ] || [ "$WORKDIR" = "/tmp" ] || [ ! -d "$WORKDIR" ]; then
  echo "ERROR: Could not create temporary working directory."
  exit 1
fi
cd $WORKDIR || { echo "Failed to change directory to $WORKDIR. Exiting."; exit 1; }
echo "Created temporary directory: $WORKDIR"

# Function to clean up the output directory and log termination
function cleanup {
  # Make sure WORKDIR is a proper temp directory so we don't rm something we shouldn't
  # shellcheck disable=SC3010
  if [[ $WORKDIR =~ ^/tmp/tmp\.[a-zA-Z0-9]+$ ]]; then
    if [ "$DEBUG" != "1" ]; then
      echo "Cleaning up $WORKDIR..."
      rm -rf "$WORKDIR"
    else
      echo "DEBUG active or $WORKDIR looks wrong; leaving $WORKDIR behind."
    fi
  else
    echo "ERROR: WORKDIR ($WORKDIR) doesn't look like a proper mktemp directory; not removing it for safety reasons!"
    exit 255
  fi
  echo "Log collection finished."
}

# Execute the cleanup function if the script terminates
trap "exit 1" HUP INT PIPE QUIT TERM
trap "cleanup" EXIT

# This function runs a command and dumps its output to a named pipe, then includes that named
# pipe into a zip file. It's used to include command output in the ZIP file without taking up
# any disk space aside from the ZIP file itself.
# USAGE: collectToZip FILENAME CMDTORUN
function collectToZip {
  command -v "${2}" >/dev/null || { printf "%s not found, skipping.\n" "${2}"; return; }
  mkfifo "${1}"
  "${@:2}" >"${1}" 2>&1 &
  zip -gumDZ deflate --fifo "${ZIP}" "${1}"
}

# Collect general system information
echo "Collecting system information..."
mkdir collect

# Collect general information and create the ZIP in the first place
zip -DZ deflate "${ZIP}" /proc/@(cmdline|cpuinfo|filesystems|interrupts|loadavg|meminfo|modules|mounts|slabinfo|stat|uptime|version*|vmstat) /proc/net/*

# Collect system information
collectToZip collect/systemctl-status.txt systemctl status --all -fr

# Collect network information
collectToZip collect/conntrack.txt conntrack -L
collectToZip collect/conntrack_stats.txt conntrack -S
collectToZip collect/ip_4_addr.json ip -4 -d -j addr show
collectToZip collect/ip_4_neighbor.json ip -4 -d -j neighbor show
collectToZip collect/ip_4_route.json ip -4 -d -j route show
collectToZip collect/ip_4_tcpmetrics.json ip -4 -d -j tcpmetrics show
collectToZip collect/ip_6_addr.json ip -6 -d -j addr show table all
collectToZip collect/ip_6_neighbor.json ip -6 -d -j neighbor show
collectToZip collect/ip_6_route.json ip -6 -d -j route show table all
collectToZip collect/ip_6_tcpmetrics.json ip -6 -d -j tcpmetrics show
collectToZip collect/ip_link.json ip -d -j link show
collectToZip collect/ip_netconf.json ip -d -j netconf show
collectToZip collect/ip_netns.json ip -d -j netns show
collectToZip collect/ip_rule.json ip -d -j rule show

# Add each file sequentially to the zip archive. This is slightly less efficient then adding them
# all at once, but allows us to easily check when we've exceeded the maximum file size and stop
# adding things to the archive.
echo "Adding log files to zip archive..."
for file in ${GLOBS[*]}; do
  if test -e "$file"; then
    if [ "${ENFORCE_MAX_ZIP_SIZE}" = "true" ]; then
      # If the archive is already at or over the max size, stop adding files.
      FILE_SIZE=$(stat --printf "%s" "${ZIP}" 2>/dev/null || echo 0)
      if [ "$FILE_SIZE" -ge "$MAX_SIZE" ]; then
        echo "WARNING: ZIP file size $FILE_SIZE >= $MAX_SIZE; not adding more files."
        break
      fi
    fi

    zip -g -DZ deflate -u "${ZIP}" "$file" -x '*.sock'

    if [ "${ENFORCE_MAX_ZIP_SIZE}" = "true" ]; then
      # The API for the log bundle has a max file size (defined above, usually 100MB), so if
      # adding this last file made the zip go over that size, remove that file and try the next one.
      # Using continue instead of break ensures smaller subsequent files can still be included.
      FILE_SIZE=$(stat --printf "%s" "${ZIP}")
      if [ "$FILE_SIZE" -ge "$MAX_SIZE" ]; then
        echo "WARNING: ZIP file size $FILE_SIZE >= $MAX_SIZE after adding $file; removing it and trying next file."
        zip -d "${ZIP}" "$file"
      fi
    fi
  fi
done

# Copy the log bundle to the expected path for uploading, then trigger
# the upload script to push it to the host storage location.
echo "Log bundle size: $(du -hs ${ZIP})"
mkdir -p /var/lib/waagent/logcollector
cp ${ZIP} /var/lib/waagent/logcollector/logs.zip
echo -n "Uploading log bundle: "
/usr/bin/env python3 /opt/azure/containers/aks-log-collector-send.py
