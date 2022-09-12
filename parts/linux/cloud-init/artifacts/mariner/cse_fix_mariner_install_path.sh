set +x


echo "Waiting for provision_configs.sh"
until [ -f /opt/azure/containers/provision_configs.sh ]
do
    :
done
echo "/opt/azure/containers/provision_configs.sh found"

sed -i 's/\(.*\)configGPUDrivers$/\1tdnf -y install cuda nvidia-container-runtime nvidia-container-toolkit libnvidia-container-tools libnvidia-container1' /opt/azure/containers/provision_configs.sh

echo "configuration successful"