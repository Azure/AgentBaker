set +x


echo "Waiting for provision_configs.sh"
until [ -f /opt/azure/containers/provision_configs.sh ]
do
    :
done
echo "/opt/azure/containers/provision_configs.sh found"

sed -i 's/\(.*\)configGPUDrivers$/\1tdnf -y install https://packages.microsoft.com/cbl-mariner/2.0/prod/nvidia/x86_64/cuda-510.47.03-3_5.15.57.1.cm2.x86_64.rpm nvidia-container-runtime nvidia-container-toolkit libnvidia-container-tools libnvidia-container1' /opt/azure/containers/provision_configs.sh

echo "configuration successful"