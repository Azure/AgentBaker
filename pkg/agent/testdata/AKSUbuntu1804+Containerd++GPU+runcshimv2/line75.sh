[Unit]
Description=a script that installs the mig-parted library to do multi-instance GPU partitioning
# After=
[Service]
Restart=on-failure
ExecStart=/bin/bash /opt/azure/containers/mig-partition.sh
#EOF