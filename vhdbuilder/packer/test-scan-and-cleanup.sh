#!/bin/bash

retrycmd_if_failure() {
  retries=${1}; wait_sleep=${2}; cmd=${3}; target=$(basename $(echo ${3}))
  echo "##[group]$target"
  echo -e "Running ${cmd} with ${retries} retries" >> ${target%.*}-output.txt
  for i in $(seq 1 ${retries}); do
    ${cmd} >> ${target%.*}-output.txt 2>&1 && break ||
    if [ ${i} -eq ${retries} ]; then
      sed -i "3i ${target} failed ${i} times" ${target%.*}-output.txt
      echo "##[endgroup]$target" >> ${target%.*}-output.txt
      cat ${target%.*}-output.txt && rm ${target%.*}-output.txt
      exit 1
    else
      sleep ${wait_sleep}
      echo -e "\n\nNext Attempt:\n\n" >> ${target%.*}-output.txt
    fi
  done
  echo "##[endgroup]$target" >> ${target%.*}-output.txt
  cat ${target%.*}-output.txt && rm ${target%.*}-output.txt
}

if [[ -z "$SIG_GALLERY_NAME" ]]; then
  SIG_GALLERY_NAME="PackerSigGalleryEastUS"
fi

SCRIPT_ARRAY+=("./vhdbuilder/packer/cleanup.sh") # Always run cleanup

# Check to ensure the build step succeeded
SIG_VERSION=$(az sig image-version show \
-e ${CAPTURED_SIG_VERSION} \
-i ${SIG_IMAGE_NAME} \
-r ${SIG_GALLERY_NAME} \
-g ${AZURE_RESOURCE_GROUP_NAME} \
--query id --output tsv || true)

if [ -z "${SIG_VERSION}" ]; then
  echo -e "\nBuild step did not produce an image version. Running cleanup and then exiting."
  retrycmd_if_failure 2 3 "${SCRIPT_ARRAY[@]}"
  EXIT_CODE=$?
  exit ${EXIT_CODE}
fi

if [ "$IMG_SKU" != "20_04-lts-cvm" ]; then
  SCRIPT_ARRAY+=("./vhdbuilder/packer/test/run-test.sh")
else
  echo -e "\n\nSkipping tests for CVM 20.04"
fi

if [ "$OS_VERSION" != "18.04" ]; then
  SCRIPT_ARRAY+=("./vhdbuilder/packer/vhd-scanning.sh")
else
  # 18.04 VMs don't have access to new enough 'az' versions to be able to run the az commands in vhd-scanning-vm-exe.sh
  echo -e "\n\nSkipping scanning for 18.04"
fi

echo -e "Running the following scripts: ${SCRIPT_ARRAY[@]}"
declare -A SCRIPT_PIDS
for SCRIPT in "${SCRIPT_ARRAY[@]}"; do
  retrycmd_if_failure 2 3 "${SCRIPT}" &
  PID=$!
  SCRIPT_PIDS[$SCRIPT]=${PID}
done
wait ${SCRIPT_PIDS[@]}

echo -e "Checking exit codes for each script..."
STEP_FAILED=false
for SCRIPT in "${!SCRIPT_PIDS[@]}"; do
  PID=${SCRIPT_PIDS[$SCRIPT]}
  wait $PID
  EXIT_CODE=$?
  if [ ${EXIT_CODE} -ne 0 ]; then
    STEP_FAILED=true
  fi
  echo -e "${SCRIPT} exited with code ${EXIT_CODE}"
done
if [[ "${STEP_FAILED}" == true ]]; then
  echo "One or more scripts failed. Exiting with exit code 1."
  exit 1
fi
echo -e "\n\n\nTest, Scan, and Cleanup script completed.\n\n\n"