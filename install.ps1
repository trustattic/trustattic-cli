#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$Repo = "trustattic/trustattic-cli"

function Get-OSName {
    return "windows"
}

function Get-ArchName {
    param([string]$ProcessorArch)
    switch ($ProcessorArch) {
        "AMD64" { return "amd64" }
        # .goreleaser.yaml's build matrix excludes windows/arm64, so there is no
        # asset to download — report it as unsupported rather than 404ing later.
        "ARM64" { return "unsupported" }
        default { return "unsupported" }
    }
}

function Get-AssetName {
    param([string]$Os, [string]$Arch)
    return "trustattic_${Os}_${Arch}.zip"
}

function Install-Trustattic {
    $os = Get-OSName
    $arch = Get-ArchName -ProcessorArch $env:PROCESSOR_ARCHITECTURE

    if ($arch -eq "unsupported") {
        Write-Error "unsupported platform (windows/$env:PROCESSOR_ARCHITECTURE). trustattic-cli supports windows/amd64."
        exit 1
    }

    $asset = Get-AssetName -Os $os -Arch $arch
    $url = "https://github.com/$Repo/releases/latest/download/$asset"
    $checksumsUrl = "https://github.com/$Repo/releases/latest/download/checksums.txt"

    $tmpDir = Join-Path $env:TEMP "trustattic-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tmpDir | Out-Null

    try {
        $assetPath = Join-Path $tmpDir $asset
        Write-Host "Downloading $asset..."
        try {
            Invoke-WebRequest -Uri $url -OutFile $assetPath -UseBasicParsing
        } catch {
            Write-Error "no release asset found for windows/$arch. tried: $url"
            exit 1
        }

        $checksumsPath = Join-Path $tmpDir "checksums.txt"
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

        Write-Host "Verifying checksum..."
        $expectedLine = Select-String -Path $checksumsPath -Pattern ([regex]::Escape($asset))
        if (-not $expectedLine) {
            Write-Error "$asset not listed in checksums.txt"
            exit 1
        }
        $expected = ($expectedLine.Line -split '\s+')[0]
        $actual = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) {
            Write-Error "checksum mismatch for ${asset}: expected $expected, got $actual"
            exit 1
        }

        Write-Host "Extracting..."
        Expand-Archive -Path $assetPath -DestinationPath $tmpDir -Force

        $installDir = Join-Path $env:LOCALAPPDATA "trustattic\bin"
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        Copy-Item -Path (Join-Path $tmpDir "trustattic.exe") -Destination (Join-Path $installDir "trustattic.exe") -Force

        Write-Host "Installed trustattic to $installDir\trustattic.exe"

        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        # Literal, delimiter-aware membership test: -like/-notlike would treat
        # $installDir as a wildcard pattern (e.g. [ ] ? * in a username).
        $pathEntries = $userPath -split ';'
        if ($pathEntries -notcontains $installDir) {
            # A user with no user-scoped PATH yet gets $null here; concatenating
            # blindly would leave a leading ';' in the new value.
            if ([string]::IsNullOrEmpty($userPath)) {
                $newPath = $installDir
            } else {
                $newPath = "$userPath;$installDir"
            }
            [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
            Write-Host "Added $installDir to your user PATH. Restart your terminal for it to take effect."
        }

        Write-Host "Run 'trustattic login' to get started."
    } finally {
        Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    }
}

# Only run when executed directly (not dot-sourced for testing).
if ($MyInvocation.InvocationName -ne '.') {
    Install-Trustattic
}
