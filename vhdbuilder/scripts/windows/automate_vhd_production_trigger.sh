#!/bin/bash
set -euxo pipefail

az login --identity

trigger_win_vhd_prod_pipeline() {
    az pipelines run --id 188674 # --name "AKS Windows VHD Build -ContainerD&Docker"    
    # This is for testing only and should be deleted.
    # az pipelines run --id 182855 # use the AKS Windows VHD Build - PR check-in gate for trial and error
}

trigger_win_vhd_prod_pipeline
