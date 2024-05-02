#! /bin/bash

LOG_DIR="/var/log/azure"

### START CONFIGURATION
ZIP="aks_debug_logs.zip"

# Log bundle upload max size is limited to 100MB
# MAX_SIZE=104857600

command -v zip >/dev/null || {
  echo "Error: zip utility not found. Please install zip."
  exit 255
}

# Create a temporary directory to store results in
WORKDIR="$(mktemp -d)"
# check if tmp dir was created
if [[ ! "$WORKDIR" || "$WORKDIR" == "/" || "$WORKDIR" == "/tmp" || ! -d "$WORKDIR" ]]; then
  echo "ERROR: Could not create temporary working directory."
  exit 1
fi
cd $WORKDIR
echo "Created temporary directory: $WORKDIR"

# Function to clean up the output directory and log termination
function cleanup {
  # Make sure WORKDIR is a proper temp directory so we don't rm something we shouldn't
  if [[ $WORKDIR =~ ^/tmp/tmp\.[a-zA-Z0-9]+$ ]]; then
    if [[ "$DEBUG" != "1" ]]; then
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
  command -v "${2}" >/dev/null || { printf "${2} not found, skipping.\n"; return; }
  mkfifo "${1}"
  ${@:2} >"${1}" 2>&1 &
  zip -gumDZ deflate --fifo "${ZIP}" "${1}"
}

# Collect general system information
echo "Collecting debug information..."
mkdir collect

# Collect cgroup information and create the ZIP in the first place
zip -DZ deflate "${ZIP}" /proc/*/cgroup

# Collect process information
collectToZip collect/ps.txt ps -auxf
collectToZip collect/cgls.txt systemd-cgls
sudo timeout 30 bpftrace -e 'tracepoint:syscalls:sys_enter_wait4 { printf("wait() called by PID %d\n", pid); }' > collect/bpftrace.txt
zip -gumDZ deflate --fifo "${ZIP}" collect/bpftrace.txt

echo "Log bundle size: $(du -hs ${ZIP})"
mkdir -p /var/lib/waagent/logcollector
cp ${ZIP} /var/lib/waagent/logcollector/logs.zip
echo -n "Uploading log bundle: "
/usr/bin/env python3 /opt/azure/containers/aks-log-collector-send.py
echo -n "Copy to log directory: "
cp ${ZIP} ${LOG_DIR}/${ZIP}
