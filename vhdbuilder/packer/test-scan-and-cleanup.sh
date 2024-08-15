#!/bin/bash

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; cmd=$3; target=$(basename $(echo $3))
    echo -e "\n\n==========================================================="
    echo -e "Running $cmd with $retries retries"
    for i in $(seq 1 $retries); do
      $cmd >> output-${target%.*}.txt 2>&1 && break ||
      if [ $i -eq $retries ]; then
        echo -e "$target failed $i times\n"
        cat output-${target%.*}.txt
        exit 1
      else
        sleep $wait_sleep
        echo -e "\n\n\nNext Attempt:\n\n\n" >> output-${target%.*}.txt
      fi
    done
  cat output-${target%.*}.txt
}

if [[ -z "$SIG_GALLERY_NAME" ]]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

TARGET_ARRAY=()
TARGET_ARRAY+=("cleanup") # Always run cleanup
MAKE_CMD_PREFIX="make -f packer.mk"

# Check to ensure the build step succeeded
SIG_VERSION=$(az sig image-version show \
-e ${CAPTURED_SIG_VERSION} \
-i ${SIG_IMAGE_NAME} \
-r ${SIG_GALLERY_NAME} \
-g ${AZURE_RESOURCE_GROUP_NAME} \
--query id --output tsv || true)

if [ -z "${SIG_VERSION}" ]; then
  echo -e "Build step did not produce an image version. Running cleanup...\n\n\n"
  retrycmd_if_failure 2 3 "${MAKE_CMD_PREFIX} ${TARGET_ARRAY[@]}"
fi

if [ "$IMG_SKU" != "20_04-lts-cvm" ]; then
  TARGET_ARRAY+=("test-building-vhd")
else
  echo -e "Skipping tests for CVM 20.04\n\n\n"
fi

if [ "$OS_VERSION" != "18.04" ]; then
  TARGET_ARRAY+=("scanning-vhd")
else
  # 18.04 VMs don't have access to new enough 'az' versions to be able to run the az commands in vhd-scanning-vm-exe.sh
  echo -e "Skipping scanning for 18.04\n\n\n"
fi

TARGET_PIDS=()
for TARGET in "${TARGET_ARRAY[@]}"; do
  retrycmd_if_failure 2 3 "${MAKE_CMD_PREFIX} ${TARGET}" &
  TARGET_PIDS+=($!)
done

wait ${TARGET_PIDS[@]}

echo -e "\n\n\nTest, Scan, and Cleanup Script completed.\n\n\n"