param(
    [string]$InstallDir = "$env:LOCALAPPDATA\StateRelay\bin",
    [switch]$AddToPath
)

$ErrorActionPreference = "Stop"

$Source = Join-Path $PSScriptRoot "relay.exe"
if (-not (Test-Path $Source)) {
    throw "relay.exe was not found next to install.ps1"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$Target = Join-Path $InstallDir "relay.exe"
Copy-Item $Source $Target -Force

if ($AddToPath) {
    $CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Parts = @()
    if ($CurrentPath) {
        $Parts = $CurrentPath -split ";"
    }
    if ($Parts -notcontains $InstallDir) {
        $NewPath = if ($CurrentPath) { "$CurrentPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        Write-Host "Added $InstallDir to the user PATH. Open a new terminal to use relay."
    }
}

Write-Host "Installed relay to $Target"
Write-Host "Run: relay version"
