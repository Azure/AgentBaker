#!/bin/sh

mkdir -p logs
rm -f logs/*

scp windows:/AzureData/provision.complete logs/
scp windows:/AzureData/CustomData.bin logs/
scp windows:/k/config logs/
scp windows:/k/bootstrap-config logs/
scp windows:/k/\*.log logs/
scp windows:/k/\*.ps1 logs/
scp windows:/AzureData/\*.log logs/
