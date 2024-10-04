#!/bin/bash

required_env_vars=(
  "SIG_IMAGE_NAME"
  "AZURE_RESOURCE_GROUP_NAME"
  "CAPTURED_SIG_VERSION"
  "ENVIRONMENT"
  "SIG_GALLERY_NAME"
  "OS_VERSION"
  "SIG_IMAGE_NAME"
  "UMSI_RESOURCE_ID"
  "UMSI_PRINCIPAL_ID"
  "UMSI_CLIENT_ID"
  "BUILD_RUN_NUMBER"
)

for v in "${required_env_vars[@]}"; do
  if [ -z "${!v}" ]; then
    echo "$v was not set!"
    exit 1
  fi
done

echo "${PWD}"
echo "SCANNING MSI RESOURCE ID set to ${SCANNING_MSI_RESOURCE_ID}"
echo "OS_DISK_URI set to ${OS_DISK_URI}" 
echo "MANAGED_SIG_ID set to ${MANAGED_SIG_ID}" 
echo "SIG_GALLERY_NAME set to ${SIG_GALLERY_NAME}" 
echo "CAPTURED_SIG_VERSION set to ${CAPTURED_SIG_VERSION}" 
echo "IMPORTED_IMAGE_NAME set to ${IMPORTED_IMAGE_NAME}" 
echo "SIG_IMAGE_NAME set to ${SIG_IMAGE_NAME}"
echo "SIG_GALLERY_NAME set to ${SIG_GALLERY_NAME}"
echo "BUILD_PERF_DATA_FILE set to ${BUILD_PERF_DATA_FILE}"
echo "IS_NOT_1804 set to ${IS_NOT_1804}"
echo "OS_TYPE set to ${OS_TYPE}"
echo "OS_NAME set to ${OS_NAME}"
echo "GIT_VERSION set to ${GIT_VERSION}" 
echo "BUILD_DEFINITION_NAME set to ${BUILD_DEFINITION_NAME}"
echo "BUILD_ID set to ${BUILD_ID}"
echo "BUILD_NUMBER set to ${BUILD_NUMBER}"
echo "BRANCH set to ${BRANCH}"
echo "GIT_BRANCH set to ${GIT_BRANCH}"
echo "BUILD_ID set to ${BUILD_ID}"
echo "JOB_STATUS set to ${JOB_STATUS}" 
echo "VHD_DEBUG set to ${VHD_DEBUG}" 
echo "PKR_RG_NAME set to ${PKR_RG_NAME}"
echo "IMAGE_NAME set to ${IMAGE_NAME}"
echo "VHD_NAME set to ${VHD_NAME}"
echo "SUBSCRIPTION_ID is set to $SUBSCRIPTION_ID"
echo "AZURE_RESOURCE_GROUP_NAME is set to $AZURE_RESOURCE_GROUP_NAME"
echo "BLOB_STORAGE_NAME is set to $BLOB_STORAGE_NAME"
echo "CLASSIC_BLOB is set to $CLASSIC_BLOB"
echo "ENVIRONMENT is set to $ENVIRONMENT"
echo "BUILD_ID is set to $BUILD_ID"
echo "MANAGED_SIG_ID is set to $MANAGED_SIG_ID"
echo "PACKER_BUILD_LOCATION is set to $PACKER_BUILD_LOCATION"
echo "OS_VERSION is set to $OS_VERSION"
echo "OS_SKU is set to $OS_SKU"
echo "PKR_RG_NAME is set to $PKR_RG_NAME"
echo "MODE is set to $MODE"
echo "DRY_RUN is set to $DRY_RUN"
echo "OS_TYPE is set to $OS_TYPE"
echo "AZURE_RESOURCE_GROUP_NAME is set to $AZURE_RESOURCE_GROUP_NAME"
echo "IMAGE_NAME is set to $IMAGE_NAME"
echo "CAPTURED_SIG_VERSION is set to $CAPTURED_SIG_VERSION"
echo "IMPORTED_IMAGE_NAME is set to $IMPORTED_IMAGE_NAME"
echo "SIG_GALLERY_NAME is set to $SIG_GALLERY_NAME"
echo "SIG_IMAGE_NAME is set to $SIG_IMAGE_NAME"
echo "ARCHITECTURE is set to $ARCHITECTURE"
echo "OS_DISK_URI is set to $OS_DISK_URI"
echo "MANAGED_SIG_ID is set to $MANAGED_SIG_ID"
echo "PACKER_BUILD_LOCATION is set to $PACKER_BUILD_LOCATION"
echo "CONTAINER_RUNTIME is set to $CONTAINER_RUNTIME"
echo "OS_VERSION is set to $OS_VERSION"
echo "OS_SKU is set to $OS_SKU"
echo "IMG_SKU is set to $IMG_SKU"
echo "VHD_DEBUG is set to $VHD_DEBUG"
echo "FEATURE_FLAGS is set to $FEATURE_FLAGS"
echo "SIG_IMAGE_VERSION is set to $SIG_IMAGE_VERSION"
echo "ENABLE_FIPS is set to $ENABLE_FIPS"
echo "ENABLE_TRUSTED_LAUNCH is set to $ENABLE_TRUSTED_LAUNCH"
echo "SGX_INSTALL is set to $SGX_INSTALL"
echo "ENABLE_CGROUPV2 is set to $ENABLE_CGROUPV2"
echo "GIT_BRANCH is set to $GIT_BRANCH"
echo "BLOB_STORAGE_NAME is set to $BLOB_STORAGE_NAME"
echo "CLASSIC_BLOB is set to $CLASSIC_BLOB"
echo "BUILD_ID is set to $BUILD_ID"
echo "SKU_NAME is set to $SKU_NAME"
echo "VHD_NAME is set to $VHD_NAME"
echo "VHD_DEBUG is set to $VHD_DEBUG"
echo "ARCHITECTURE is set to $ARCHITECTURE"
echo "KUSTO_ENDPOINT is set to $KUSTO_ENDPOINT"
echo "KUSTO_DATABASE is set to $KUSTO_DATABASE"
echo "KUSTO_TABLE is set to $KUSTO_TABLE"
echo "UMSI_RESOURCE_ID is set to $UMSI_RESOURCE_ID"
echo "UMSI_PRINCIPAL_ID is set to $UMSI_PRINCIPAL_ID"
echo "UMSI_CLIENT_ID is set to $UMSI_CLIENT_ID"
echo "ACCOUNT_NAME is set to $ACCOUNT_NAME"
echo "BLOB_URL is set to $BLOB_URL"
echo "SEVERITY is set to $SEVERITY"
echo "MODULE_VERSION is set to $MODULE_VERSION"
echo "BUILD_REPOSITORY_NAME is set to $BUILD_REPOSITORY_NAME"
echo "BUILD_SOURCEVERSION is set to $BUILD_SOURCEVERSION"
echo "SYSTEM_COLLECTIONURI is set to $SYSTEM_COLLECTIONURI"
echo "SYSTEM_TEAMPROJECT is set to $SYSTEM_TEAMPROJECT"
echo "BUILD_RUN_NUMBER is set to $BUILD_RUN_NUMBER"

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

# Always run cleanup
SCRIPT_ARRAY+=("./vhdbuilder/packer/cleanup.sh")

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

# Setup testing
SCRIPT_ARRAY+=("./vhdbuilder/packer/test/run-test.sh")

# Setup scanning
echo -e "\nENVIRONMENT is: ${ENVIRONMENT}, OS_VERSION is: ${OS_VERSION}"
if [ "${ENVIRONMENT,,}" != "prod" ] && [ "$OS_VERSION" != "18.04" ]; then
  echo -e "Running scanning step"
  SCRIPT_ARRAY+=("./vhdbuilder/packer/vhd-scanning.sh")
else
  echo -e "Skipping scanning step"
fi

echo -e "\nRunning the following scripts: ${SCRIPT_ARRAY[@]}\n"
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