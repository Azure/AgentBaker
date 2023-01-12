Invoke-Pester -Output Detailed $PSScriptRoot\azurecnifunc.tests.ps1
Invoke-Pester -Output Detailed $PSScriptRoot\configfunc.tests.ps1
Invoke-Pester -Output Detailed $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.tests.ps1