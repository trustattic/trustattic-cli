BeforeAll {
    . "$PSScriptRoot/../../install.ps1"
}

Describe "Get-OSName" {
    It "returns windows" {
        Get-OSName | Should -Be "windows"
    }
}

Describe "Get-ArchName" {
    It "maps AMD64 to amd64" {
        Get-ArchName -ProcessorArch "AMD64" | Should -Be "amd64"
    }
    It "maps ARM64 to unsupported (not in the build matrix)" {
        Get-ArchName -ProcessorArch "ARM64" | Should -Be "unsupported"
    }
    It "maps an unknown arch to unsupported" {
        Get-ArchName -ProcessorArch "IA64" | Should -Be "unsupported"
    }
}

Describe "Get-AssetName" {
    It "builds the windows amd64 asset name" {
        Get-AssetName -Os "windows" -Arch "amd64" | Should -Be "trustattic_windows_amd64.zip"
    }
}
