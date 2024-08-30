#!/bin/bash
set -e

export subID=subID
export tenantId=tenantId

export fqdn=https://aks-timmy-wrightt-resource-82acd5-zr3wop33.hcp.australiaeast.azmk8s.io
export WINDOWS_E2E_VMSIZE=Standard_DS1_v2
export KUBERNETES_VERSION=1.30.3
export csePackageURL=https://acs-mirror.azureedge.net/aks/windows/cse/

## From vhdbuilder/packer/generate-windows-vhd-configuration.ps1 line 176
#        "https://acs-mirror.azureedge.net/kubernetes/v1.27.14-hotfix.20240712/windowszip/v1.27.14-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.27.15-hotfix.20240712/windowszip/v1.27.15-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.27.16/windowszip/v1.27.16-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.5-hotfix.20240712/windowszip/v1.28.5-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.9-hotfix.20240712/windowszip/v1.28.9-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.10-hotfix.20240712/windowszip/v1.28.10-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.11-hotfix.20240712/windowszip/v1.28.11-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.12/windowszip/v1.28.12-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.13/windowszip/v1.28.13-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.2-hotfix.20240712/windowszip/v1.29.2-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.4-hotfix.20240712/windowszip/v1.29.4-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.5-hotfix.20240712/windowszip/v1.29.5-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.6-hotfix.20240712/windowszip/v1.29.6-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.7/windowszip/v1.29.7-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.8/windowszip/v1.29.8-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.1-hotfix.20240712/windowszip/v1.30.1-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.2-hotfix.20240712/windowszip/v1.30.2-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.3/windowszip/v1.30.3-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.4/windowszip/v1.30.4-1int.zip"
export kubeBinariesSASURL=https://acs-mirror.azureedge.net/kubernetes/v1.30.3/windowszip/v1.30.3-1int.zip

export WINDOWS_E2E_IMAGE=2019-containerd
export WINDOWS_DISTRO=aks-windows-2019-containerd

export WINDOWS_GPU_DRIVER_SUFFIX=
export WINDOWS_GPU_DRIVER_URL=""
export CONFIG_GPU_DRIVER_IF_NEEDED=false

export caCrt="LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURIVENDQWdXZ0F3SUJBZ0lRQ1RyYng3N1ZrbHh2WjlBTDRQUkdnVEFOQmdrcWhraUc5dzBCQVFzRkFEQU4KTVFzd0NRWURWUVFERXdKallUQWVGdzB5TkRBNE1qRXlNRE15TWpKYUZ3MHlOakE0TWpFeU1EUXlNakphTURBeApGekFWQmdOVkJBb1REbk41YzNSbGJUcHRZWE4wWlhKek1SVXdFd1lEVlFRREV3eHRZWE4wWlhKamJHbGxiblF3CmdnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM0eXpWLzJDZWY0TnNKV3Z3eWN1Q0EKT3FkTWIwRFY3ZXVldjJ5K05hT1E3N09UNWJjS01MeVJpMDhTa3NmVU9FaWhBTmNaT0tDVXNWa3BiTXFiWXYrbQpidUU1WGI2TklzSTducTgrakxHckF1WVY5dU5mbGdxZ2Frc2w2UDBFT2pXcVVqaVovOGJPYnd2cXR1a2hUL2tzCmFwaVJPQ3hwY0NrSlZvc3BONUJFbSs1N2ttR3lBdElBc0FhTWJqN3RZTU52bmEvYm5na0FmVVZXc1Q2REhUcFMKeUFEdTNNY3djZCtqODlSV0FaNUFnWEhZdElNSlJ3STFJTkUvTzdpOVh5ZWZ0VStXVE1yc2g4M3hlMUxUVzBISgpWTjhFRGlvaUJOLzJTN01qMXhZSUhxbUFmN29JNUE5OXlraVRyL0lyYjFORFIrY2F3RGVmb2FmckFZekxPR2ZuCkFnTUJBQUdqVmpCVU1BNEdBMVVkRHdFQi93UUVBd0lGb0RBVEJnTlZIU1VFRERBS0JnZ3JCZ0VGQlFjREFqQU0KQmdOVkhSTUJBZjhFQWpBQU1COEdBMVVkSXdRWU1CYUFGSGtOdXhWNmxnUzg1QTBxVWgrTjlUWlgwVWJzTUEwRwpDU3FHU0liM0RRRUJDd1VBQTRJQkFRQzdROThjUE5RK2RxVkszNThPS1FHclhjUXZlUXRXNkhyVXRXNGVLdnY4Ci9FUnp1MHVvVWFVbnlQWHRNd20rcG5Fd3JhS2ZLUmd6aWg1S25xbURzKzk0aUlqN3ZOTURqcHJKblZRZG5OWTUKb011Z1FtUVN1RjRTSUl0Qzl3KzdmaHNKSGRPeURvOEZpaWhJb0F6Zm5EMkw3VkY0d0dJaTlFNU1qdVNrS0FtKwpaanNNU2pSYzNMREdvTWUzdzhNQllLTEQwcS91SjVBT0JRUmZxUTZoaWdEVEZ5NzVPWEhiaS9sVmhJbUFTVDkzCnhLbndjTjl3NFJsSTI5enBrUEtVbzNQOXhaaWRMemk0Y3ozOFIwQ3FOYmhBR3dGdFhUSWNwUWxreUVpWm9rM3oKV0dGYzUzYUZ2TWlmTG9qQ2NaSkMrbnNXcjFRWkhUT2JuTnlYOXpPQUlDdDEKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo="
export apiserverCrt=$caCrt

export adminUserName=tim
export adminPublicKeyData="public key data"

envsubst < percluster_template.json | jq -s '.[0] * .[1]' nodebootstrapping_static.json - | go run main.go getCustomScript  > CustomScriptExtension.bat
envsubst < percluster_template.json | jq -s '.[0] * .[1]' nodebootstrapping_static.json - | go run main.go getCustomScriptData | base64 --decode > CustomData.bin

scp CustomScriptExtension.bat CustomData.bin tim@timmy-win-vm.australiaeast.cloudapp.azure.com:/AzureData/
