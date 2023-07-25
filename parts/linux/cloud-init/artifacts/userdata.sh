#!/bin/bash
set -x
curl -H Metadata:true --noproxy "*" "http://169.254.169.254/metadata/instance/compute/userData?api-version=2021-01-01&format=text" | base64 -d > csecmd.sh
chmod +x csecmd.sh
cat csecmd.sh
./csecmd.sh