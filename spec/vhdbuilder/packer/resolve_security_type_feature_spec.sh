#!/bin/bash

Describe 'resolve_security_type_feature'
  Include './vhdbuilder/packer/produce-packer-settings-functions.sh'

  Parameters
    False True None TrustedLaunchSupported
    True True None TrustedLaunch
    True False None TrustedLaunch
    False False cvm ConfidentialVMSupported
    False False None Standard
    true true None TrustedLaunch
    false true None TrustedLaunchSupported
  End

  It "resolves enabled=$1 supported=$2 flags=$3 to $4"
    ENABLE_TRUSTED_LAUNCH="$1"
    TRUSTED_LAUNCH_SUPPORTED="$2"
    FEATURE_FLAGS="$3"

    When call resolve_security_type_feature
    The status should be success
    The variable SECURITY_TYPE_FEATURE should equal "$4"
  End

  It 'reports native Confidential VM capture for ACL CVM rather than supported-only'
    OS_SKU=AzureContainerLinux
    ENABLE_TRUSTED_LAUNCH=False
    TRUSTED_LAUNCH_SUPPORTED=False
    FEATURE_FLAGS=cvm

    When call resolve_security_type_feature
    The status should be success
    The variable SECURITY_TYPE_FEATURE should equal ConfidentialVM
  End
End
