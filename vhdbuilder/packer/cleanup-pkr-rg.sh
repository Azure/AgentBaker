#!/bin/bash -eux

if [ -z "$PKR_RG_NAME" ]; then
    echo "must provide a PKR_RG_NAME env var"
    exit 1;
fi

 id=$(az group show --name ${PKR_RG_NAME})
 if [ ! -z "$id" ] ; then
   echo "Deleting packer resource group ${PKR_RG_NAME}"
   az group delete --name ${PKR_RG_NAME}
 fi