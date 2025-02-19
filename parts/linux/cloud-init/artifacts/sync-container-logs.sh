#! /bin/bash

SRC=/var/log/containers
DST=/var/log/azure/aks/pods

# Bring in OS-related bash vars
source /etc/os-release

# Install inotify-tools if they're missing from the image
if [[ ${ID} == "mariner" ]] || [[ ${ID} == "azurelinux" ]]; then
  command -v inotifywait >/dev/null 2>&1 || dnf install -y inotify-tools
else 
  command -v inotifywait >/dev/null 2>&1 || apt-get -o DPkg::Lock::Timeout=300 -y install inotify-tools
fi

# Set globbing options so that compgen grabs only the logs we want
shopt -s extglob
shopt -s nullglob

# Wait for /var/log/containers to exist
if [ ! -d $SRC ]; then
  echo -n "Waiting for $SRC to exist..."
  while [ ! -d $SRC ]; do
    sleep 15
    echo -n "."
  done
  echo "done."
fi

# Make the destination directory if not already present
mkdir -p $DST

# Start a background process to clean up logs from deleted pods that
# haven't been modified in 2 hours. This allows us to retain pod
# logs after a restart.
while true; do
  find /var/log/azure/aks/pods -type f -links 1 -mmin +120 -delete
  sleep 3600
done &

# Manually sync all matching logs once
for CONTAINER_LOG_FILE in $(compgen -G "$SRC/*_@(kube-system|tigera-operator|calico-system)_*.log"); do
   echo "Linking $CONTAINER_LOG_FILE"
   /bin/ln -Lf $CONTAINER_LOG_FILE $DST/
done
echo "Starting inotifywait..."

# Monitor for changes
inotifywait -q -m -r -e delete,create $SRC | while read DIRECTORY EVENT FILE; do
    case $FILE in
        *_@(kube-system|tigera-operator|calico-system)_*.log)
            case $EVENT in
                CREATE*)
                    echo "Linking $FILE"
                    /bin/ln -Lf "$DIRECTORY/$FILE" "$DST/$FILE"
                    ;;
            esac;;
    esac
done
