# This file uses Pester's built-in mocking capabilities to test containerdfunc.ps1

Describe "Containerd Template Selection Tests" {
    BeforeAll {
        # Load the script we're testing
        . "$PSScriptRoot\containerdfunc.ps1"

        function Write-Log {
            param([string]$Message)
            # Add to our collection for test assertions
            $global:WriteLogCalls += $Message

            Write-Host "LOG: $Message"
        }    
    }
    
    Context "GetContainerdTemplatePath function" {
        # Create a test case for each version we want to test
        $testCases = @(
            @{ Version = "1.33.0"; ExpectedTemplate = "containerd2template.toml" }
            @{ Version = "1.34.1"; ExpectedTemplate = "containerd2template.toml" }
            @{ Version = "1.32.5"; ExpectedTemplate = "containerdtemplate.toml" }
            @{ Version = "1.30.0"; ExpectedTemplate = "containerdtemplate.toml" }
        )
        
        It "Should select the <ExpectedTemplate> template for Kubernetes <Version>" -TestCases $testCases {
            param($Version, $ExpectedTemplate)
            
            # Mock the Get-KubernetesVersion function to return our test version
            Mock Get-KubernetesVersion { return $Version }
            
            # Call the function we're testing
            $result = GetContainerdTemplatePath
            
            # Verify the expected template was selected
            $result | Should -BeLike "*\$ExpectedTemplate"
            
            # Verify our mock was called
            Should -Invoke -CommandName Get-KubernetesVersion -Times 1
        }
    }
}
