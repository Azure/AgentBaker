#!/bin/bash
COMPONENTS_FILEPATH=/opt/azure/components.json

testFilesDownloaded() {
  test="testFilesDownloaded"
  echo "$test:Start"
  filesToDownload=$(cat $COMPONENTS_FILEPATH)

  filesToDownload=$(echo $filesToDownload | jq -r ".[]" | jq . --monochrome-output --compact-output)

  for fileToDownload in ${filesToDownload[*]}; do
    fileName=$(echo "${fileToDownload}" | jq .fileName -r)
    downloadLocation=$(echo "${fileToDownload}" | jq .downloadLocation -r)
    versions=$(echo "${fileToDownload}" | jq .versions -r | jq -r ".[]")
    download_URL=$(echo "${fileToDownload}" | jq .downloadURL -r)

    if [ ! -d $downloadLocation ]; then
      err $test "Directory ${downloadLocation} does not exist"
      continue
    fi

    for version in ${versions}; do
      file_Name=$(string_replace $fileName $version)
      dest="$downloadLocation/${file_Name}"
      downloadURL=$(string_replace $download_URL $version)/$file_Name

      if [ ! -s $dest ]; then
        err $test "File ${dest} does not exist"
        continue
      fi

      fileSizeInRepo=$(curl -sI $downloadURL | grep -i Content-Length | awk '{print $2}' | tr -d '\r')
      fileSizeDownloaded=$(wc -c $dest | awk '{print $1}' | tr -d '\r')
      if [[ "$fileSizeInRepo" != "$fileSizeDownloaded" ]]; then
        err $test "File size of ${dest} is invalid. Expected file size: ${fileSizeInRepo} - downlaoded file size: ${fileSizeDownloaded}"
        continue
      fi
    done

    echo "---"
  done
  echo "$test:Finish"
}

testImagesPulled() {
  test="testImagesPulled"
  echo "$test:Start"
  containerRuntime=$1
  imagesToBePulled=$(cat $COMPONENTS_FILEPATH)

  if [ $containerRuntime == 'containerd' ]; then
    pulledImages=$(ctr -n k8s.io image ls)
  elif [ $containerRuntime == 'docker' ]; then
    pulledImages=$(docker images --format "{{.Repository}}:{{.Tag}}")
  else
    err $test "unsupported container runtime $containerRuntime"
    return
  fi

  imagesNotPulled=()

  imagesToBePulled=$(echo $imagesToBePulled | jq ".ContainerImages" | jq .[] --monochrome-output --compact-output)

  for imageToBePulled in ${imagesToBePulled[*]}; do
    downloadURL=$(echo "${imageToBePulled}" | jq .downloadURL -r)
    versions=$(echo "${imageToBePulled}" | jq .versions -r | jq -r ".[]")
    for version in ${versions}; do
      download_URL=$(string_replace $downloadURL $version)

      if [[ $pulledImages =~ $downloadURL ]]; then
        echo "Image ${download_URL} has been pulled Successfully"
      else
        err $test "Image ${download_URL} has NOT been pulled"
        imagesNotPulled+=("$download_URL")
      fi
    done

    echo "---"
  done
  echo "$test:Finish"
}

testAuditDNotPresent() {
  test="testAuditDNotPresent"
  echo "$test:Start"
  status=$(systemctl show -p SubState --value auditd.service)
  if [ $status == 'dead' ]; then
    echo "AuditD is not present, as expected"
  else
    err $test "AuditD is active with status ${status}"
  fi
  echo "$test:Finish"
}

err() {
  echo "$1:Error: $2" >>/dev/stderr
}

string_replace() {
  echo ${1//\*/$2}
}

testFilesDownloaded
testImagesPulled $1
testAuditDNotPresent
