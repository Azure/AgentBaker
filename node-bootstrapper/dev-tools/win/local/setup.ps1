
filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($Message) {
    $msg = $message | Timestamp
    Write-Output $msg
}

function Update-WindowsFeatures {
    $featuresToEnable = @(
        "Containers",
        "Hyper-V",
        "Hyper-V-PowerShell")

    foreach ($feature in $featuresToEnable) {
        Write-Log "Enabling Windows feature: $feature"
        Install-WindowsFeature $feature
    }
}

function Enable-WindowsFixInFeatureManagement {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Name,
        [Parameter(Mandatory = $false)][string]
        $Value = "1",
        [Parameter(Mandatory = $false)][string]
        $Type = "DWORD"
    )

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft"
    }

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement"
    }

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides"
    }

    $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in FeatureManagement\Overrides is $currentValue"
    }
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name $Name -Value $Value -Type $Type
}

function Enable-WindowsFixInHnsState {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Name,
        [Parameter(Mandatory = $false)][string]
        $Value = "1",
        [Parameter(Mandatory = $false)][string]
        $Type = "DWORD"
    )

    $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in hns\State is $currentValue"
    }
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name $Name -Value $Value -Type $Type
}

function Enable-WindowsFixInVfpExtParameters {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Name,
        [Parameter(Mandatory = $false)][string]
        $Value = "1",
        [Parameter(Mandatory = $false)][string]
        $Type = "DWORD"
    )

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
    }

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
    }

    $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in VfpExt\Parameters is $currentValue"
    }
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name $Name $Value -Type $Type
}

function Enable-WindowsFixInPath {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Path,
        [Parameter(Mandatory = $true)][string]
        $Name,
        [Parameter(Mandatory = $false)][string]
        $Value = "1",
        [Parameter(Mandatory = $false)][string]
        $Type = "DWORD"
    )
    $regPath=(Get-Item -Path $Path -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating $Path"
        New-Item -Path $Path
    }
    $currentValue=(Get-ItemProperty -Path $Path -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in $Path is $currentValue"
    }
    Set-ItemProperty -Path $Path -Name $Name $Value -Type $Type
}

# If you need to add registry key in this function,
# please update $wuRegistryKeys and $wuRegistryNames in vhdbuilder/packer/write-release-notes-windows.ps1 at the same time
function Update-Registry {
    # Enables DNS resolution of SMB shares for containerD
    # https://github.com/kubernetes-sigs/windows-gmsa/issues/30#issuecomment-802240945
    Write-Log "Apply SMB Resolution Fix for containerD"
    Enable-WindowsFixInHnsState -Name EnableCompartmentNamespace

    if ($env:WindowsSKU -Like '2019*') {
        Write-Log "Keep the HNS fix (0x10) even though it is enabled by default. Windows are still using HNSControlFlag and may need it in the future."
        $hnsControlFlag=0x10
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HNSControlFlag is $currentValue"
            $hnsControlFlag=([int]$currentValue.HNSControlFlag -bor $hnsControlFlag)
        }
        Enable-WindowsFixInHnsState -Name HNSControlFlag -Value $hnsControlFlag

        Write-Log "Enable a WCIFS fix in 2022-10B"
        Enable-WindowsFixInPath -Path "HKLM:\SYSTEM\CurrentControlSet\Services\wcifs" -Name WcifsSOPCountDisabled -Value 0

        Write-Log "Enable 3 fixes in 2023-04B"
        Enable-WindowsFixInHnsState -Name HnsPolicyUpdateChange
        Enable-WindowsFixInHnsState -Name HnsNatAllowRuleUpdateChange
        Enable-WindowsFixInFeatureManagement -Name 3105872524

        Write-Log "Enable 1 fix in 2023-05B"
        Enable-WindowsFixInVfpExtParameters -Name VfpEvenPodDistributionIsEnabled

        Write-Log "Enable 1 fix in 2023-06B"
        Enable-WindowsFixInFeatureManagement -Name 3230913164

        Write-Log "Enable 1 fix in 2023-10B"
        Enable-WindowsFixInVfpExtParameters -Name VfpNotReuseTcpOneWayFlowIsEnabled

        Write-Log "Enable 4 fixes in 2023-11B"
        Enable-WindowsFixInHnsState -Name CleanupReservedPorts
        Enable-WindowsFixInFeatureManagement -Name 652313229
        Enable-WindowsFixInFeatureManagement -Name 2059235981
        Enable-WindowsFixInFeatureManagement -Name 3767762061

        Write-Log "Enable 1 fix in 2024-01B"
        Enable-WindowsFixInFeatureManagement -Name 1102009996

        Write-Log "Enable 2 fixes in 2024-04B"
        Enable-WindowsFixInFeatureManagement -Name 2290715789
        Enable-WindowsFixInFeatureManagement -Name 3152880268

        Write-Log "Enable 1 fix in 2024-06B"
        Enable-WindowsFixInFeatureManagement -Name 1605443213
    }

    if ($env:WindowsSKU -Like '2022*') {
        Write-Log "Enable a WCIFS fix in 2022-10B"
        Enable-WindowsFixInFeatureManagement -Name 2629306509

        Write-Log "Enable 4 fixes in 2023-04B"
        Enable-WindowsFixInHnsState -Name HnsPolicyUpdateChange
        Enable-WindowsFixInHnsState -Name HnsNatAllowRuleUpdateChange
        Enable-WindowsFixInHnsState -Name HnsAclUpdateChange
        Enable-WindowsFixInFeatureManagement -Name 3508525708

        Write-Log "Enable 4 fixes in 2023-05B"
        Enable-WindowsFixInHnsState -Name HnsNpmRefresh
        Enable-WindowsFixInVfpExtParameters -Name VfpEvenPodDistributionIsEnabled
        Enable-WindowsFixInFeatureManagement -Name 1995963020
        Enable-WindowsFixInFeatureManagement -Name 189519500

        Write-Log "Enable 1 fix in 2023-06B"
        Enable-WindowsFixInFeatureManagement -Name 3398685324

        Write-Log "Enable 4 fixes in 2023-07B"
        Enable-WindowsFixInHnsState -Name HnsNodeToClusterIpv6
        Enable-WindowsFixInHnsState -Name HNSNpmIpsetLimitChange
        Enable-WindowsFixInHnsState -Name HNSLbNatDupRuleChange
        Enable-WindowsFixInVfpExtParameters -Name VfpIpv6DipsPrintingIsEnabled

        Write-Log "Enable 3 fixes in 2023-08B"
        Enable-WindowsFixInHnsState -Name HNSUpdatePolicyForEndpointChange
        Enable-WindowsFixInHnsState -Name HNSFixExtensionUponRehydration
        Enable-WindowsFixInFeatureManagement -Name 87798413

        Write-Log "Enable 4 fixes in 2023-09B"
        Enable-WindowsFixInHnsState -Name RemoveSourcePortPreservationForRest
        Enable-WindowsFixInFeatureManagement -Name 4289201804
        Enable-WindowsFixInFeatureManagement -Name 1355135117
        Enable-WindowsFixInFeatureManagement -Name 2214038156

        Write-Log "Enable 3 fixes in 2023-10B"
        Enable-WindowsFixInHnsState -Name FwPerfImprovementChange
        Enable-WindowsFixInVfpExtParameters -Name VfpNotReuseTcpOneWayFlowIsEnabled
        Enable-WindowsFixInFeatureManagement -Name 1673770637

        Write-Log "Enable 4 fixes in 2023-11B"
        Enable-WindowsFixInHnsState -Name CleanupReservedPorts

        Enable-WindowsFixInFeatureManagement -Name 527922829
        # Then based on 527922829 to set DeltaHivePolicy=2: use delta hives, and stop generating rollups
        Enable-WindowsFixInPath -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Windows Containers" -Name DeltaHivePolicy -Value 2

        Enable-WindowsFixInFeatureManagement -Name 2193453709
        Enable-WindowsFixInFeatureManagement -Name 3331554445

        Write-Log "Enable 2 fixes in 2024-01B"
        Enable-WindowsFixInHnsState -Name OverrideReceiveRoutingForLocalAddressesIpv4
        Enable-WindowsFixInHnsState -Name OverrideReceiveRoutingForLocalAddressesIpv6

        Write-Log "Enable 1 fix in 2024-02B"
        Enable-WindowsFixInFeatureManagement -Name 1327590028

        Write-Log "Enable 4 fixes in 2024-03B"
        Enable-WindowsFixInHnsState -Name HnsPreallocatePortRange
        Enable-WindowsFixInFeatureManagement -Name 1114842764
        Enable-WindowsFixInFeatureManagement -Name 4154935436
        Enable-WindowsFixInFeatureManagement -Name 124082829

        Write-Log "Enable 11 fixes in 2024-04B"
        Enable-WindowsFixInFeatureManagement -Name 3744292492
        Enable-WindowsFixInFeatureManagement -Name 3838270605
        Enable-WindowsFixInFeatureManagement -Name 851795084
        Enable-WindowsFixInFeatureManagement -Name 26691724
        Enable-WindowsFixInFeatureManagement -Name 3834988172
        Enable-WindowsFixInFeatureManagement -Name 1535854221
        Enable-WindowsFixInFeatureManagement -Name 3632636556
        Enable-WindowsFixInFeatureManagement -Name 1552261773
        Enable-WindowsFixInFeatureManagement -Name 4186914956
        Enable-WindowsFixInFeatureManagement -Name 3173070476
        Enable-WindowsFixInFeatureManagement -Name 3958450316

        Write-Log "Enable 3 fixes in 2024-06B"
        Enable-WindowsFixInFeatureManagement -Name 2540111500
        Enable-WindowsFixInFeatureManagement -Name 50261647
        Enable-WindowsFixInFeatureManagement -Name 1475968140

        Write-Log "Enable 1 fix in 2024-07B"
        Enable-WindowsFixInFeatureManagement -Name 747051149

        Write-Log "Enable 1 fix in 2024-08B"
        Enable-WindowsFixInFeatureManagement -Name 260097166

        Write-Log "Enable 1 fix in 2024-09B"
        Enable-WindowsFixInFeatureManagement -Name 4288867982
    }

    if ($env:WindowsSKU -Like '23H2*') {
        Write-Log "Disable port exclusion change in 23H2"
        Enable-WindowsFixInHnsState -Name PortExclusionChange -Value 0

        Write-Log "Enable 1 fix in 2024-08B"
        Enable-WindowsFixInFeatureManagement -Name 1800977551
    }
}

# Update-WindowsFeatures

$env:WindowsSKU = "2022Server"
# Update-Registry

Write-Log "To use:"
Write-Log "1. Call the 'Update-WindowsFeatures' function"
Write-Log "2. Restart the machine"
Write-Log "3. Call the 'Update-Registry' function"
Write-Log "4. Restart the machine"
