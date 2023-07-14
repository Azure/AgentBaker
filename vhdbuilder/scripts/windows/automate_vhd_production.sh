#!/bin/bash
set -euxo pipefail

trigger_pipeline() {
    az pipelines run --id 188674 # --name "AKS Windows VHD Build -ContainerD&Docker"    
}
trigger_pipeline