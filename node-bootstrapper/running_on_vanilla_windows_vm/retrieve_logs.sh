#!/bin/sh

HOST=windows2

mkdir -p logs
rm -f logs/*

scp ${HOST}:/AzureData/provision.complete logs/
scp ${HOST}:/AzureData/CustomData.bin logs/
scp ${HOST}:/k/config logs/
scp ${HOST}:/k/bootstrap-config logs/
scp ${HOST}:/k/\*.log logs/
scp ${HOST}:/k/\*.ps1 logs/
scp ${HOST}:/AzureData/\*.log logs/
