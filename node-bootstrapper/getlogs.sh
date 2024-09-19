#!/bin/sh

mkdir -p logs

scp windows:/AzureData/provision.complete logs/
scp windows:/AzureData/CustomData.bin logs/
scp windows:/k/windowsnodereset.ps1 logs/
scp windows:/k/cleanupnetwork.ps1 logs/
scp windows:/k/\*.log logs/
scp windows:/AzureData/\*.log logs/
