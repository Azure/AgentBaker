#!/bin/bash -e

required_env_vars=(
    "subscription_id"
    "resource_group_name"
    "create_time"
    "location"
)


for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

echo "COPY ME ---> "

cat <<EOF > vhd-publishing-info.json
{
    "resource_id": "/subscriptions/${subscription_id}/resourceGroups/${resource_group_name}/providers/Microsoft.Compute/galleries/PackerSigGallery/images/1804Gen2/versions/1.0.${create_time}",
    "replication_regions":"$location"
}
EOF

cat vhd-publishing-info.json