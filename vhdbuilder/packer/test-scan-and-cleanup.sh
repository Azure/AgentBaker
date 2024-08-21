#!/bin/bash

retrycmd_if_failure() {
  RETRIES=${1}; WAIT_SLEEP=${2}; CMD=${3}; TARGET=$(basename ${3} .sh)
  echo "##[group]$TARGET" >> ${TARGET}-output.txt
  echo -e "Running ${CMD} with ${RETRIES} retries" >> ${TARGET}-output.txt
  for i in $(seq 1 ${RETRIES}); do
    ${CMD} >> ${TARGET}-output.txt 2>&1 && break ||
    if [ ${i} -eq ${RETRIES} ]; then
      sed -i "3i ${TARGET} failed ${i} times" ${TARGET}-output.txt
      echo "##[endgroup]$TARGET" >> ${TARGET}-output.txt
      cat ${TARGET}-output.txt && rm ${TARGET}-output.txt
      exit 1
    else
      sleep ${WAIT_SLEEP}
      echo -e "\n\nNext Attempt:\n\n" >> ${TARGET}-output.txt
    fi
  done
  echo "##[endgroup]$TARGET" >> ${TARGET}-output.txt
  cat ${TARGET}-output.txt && rm ${TARGET}-output.txt
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
--query id --output tsv)

if [ -z "${SIG_VERSION}" ]; then
  echo -e "\nBuild step did not produce an image version. Running cleanup and then exiting."
  retrycmd_if_failure 2 3 "${SCRIPT_ARRAY[@]}"
  exit $?
fi

if [ "$IMG_SKU" != "20_04-lts-cvm" ]; then
  SCRIPT_ARRAY+=("./vhdbuilder/packer/test/run-test.sh")
else
  echo -e "\n\nSkipping tests for CVM 20.04"
fi

if [ "$OS_VERSION" != "18.04" ] && [ "$OS_VERSION" != "V3" ]; then
  SCRIPT_ARRAY+=("./vhdbuilder/packer/vhd-scanning.sh")
else
  # 18.04 VMs don't have access to new enough 'az' versions to be able to run the az commands in vhd-scanning-vm-exe.sh
  # Azurelinux 3.0 does not have an 'az' package - https://microsoft.visualstudio.com/OS/_workitems/edit/52283489
  echo -e "\n\nSkipping scanning for 18.04/AzureLinux 3.0"
fi

echo -e "Running the following scripts: ${SCRIPT_ARRAY[@]}\n"
declare -A SCRIPT_PIDS
for SCRIPT in "${SCRIPT_ARRAY[@]}"; do
  retrycmd_if_failure 2 3 "${SCRIPT}" &
  PID=$!
  SCRIPT_PIDS[$SCRIPT]=${PID}
done

echo -e "\nChecking exit codes for each script..."
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
  echo -e "\nOne or more scripts failed. Exiting with exit code 1.\n"
  exit 1
fi

echo -e "\nTest, Scan, and Cleanup script successfully completed.\n"