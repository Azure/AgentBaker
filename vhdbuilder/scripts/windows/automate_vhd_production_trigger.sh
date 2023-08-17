#!/bin/bash
set -euxo pipefail

az login --identity

trigger_win_vhd_prod_pipeline() {
    az pipelines run --id 188674 # --name "AKS Windows VHD Build -ContainerD&Docker"    
}

trigger_win_vhd_prod_pipeline