$ErrorActionPreference = "Stop"

$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")
$DistDir = Join-Path $RootDir "dist"
$Version = if ($env:VERSION) {
    $env:VERSION
}
else {
    try {
        git -C $RootDir describe --tags --always --dirty
    }
    catch {
        "dev"
    }
}
$Checksums = Join-Path $DistDir "checksums.txt"

New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
New-Item -ItemType File -Force -Path $Checksums | Out-Null
Clear-Content -Path $Checksums

$Targets = @(
    @{ GOOS = "darwin"; GOARCH = "amd64"; Extension = "" },
    @{ GOOS = "darwin"; GOARCH = "arm64"; Extension = "" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Extension = "" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Extension = "" },
    @{ GOOS = "windows"; GOARCH = "amd64"; Extension = ".exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Extension = ".exe" }
)

Push-Location $RootDir
try {
    foreach ($Target in $Targets) {
        $PackageName = "staterelay-$Version-$($Target.GOOS)-$($Target.GOARCH)"
        $PackageDir = Join-Path $DistDir $PackageName
        New-Item -ItemType Directory -Force -Path $PackageDir | Out-Null

        $env:CGO_ENABLED = "0"
        $env:GOOS = $Target.GOOS
        $env:GOARCH = $Target.GOARCH

        $Output = Join-Path $PackageDir "relay$($Target.Extension)"
        Write-Host "Building $Output"
        go build -trimpath -ldflags "-s -w" -o $Output ./cmd/relay

        Copy-Item (Join-Path $RootDir "README.md") (Join-Path $PackageDir "README.md") -Force
        Copy-Item (Join-Path $RootDir "LICENSE") (Join-Path $PackageDir "LICENSE") -Force
        Set-Content -Path (Join-Path $PackageDir "VERSION.txt") -Value $Version

        if ($Target.GOOS -eq "windows") {
            Copy-Item (Join-Path $RootDir "scripts/install.ps1") (Join-Path $PackageDir "install.ps1") -Force
            $Archive = Join-Path $DistDir "$PackageName.zip"
            Compress-Archive -Path $PackageDir -DestinationPath $Archive -Force
        }
        else {
            Copy-Item (Join-Path $RootDir "scripts/install.sh") (Join-Path $PackageDir "install.sh") -Force
            $Archive = Join-Path $DistDir "$PackageName.tar.gz"
            Push-Location $DistDir
            try {
                tar -czf "$PackageName.tar.gz" $PackageName
            }
            finally {
                Pop-Location
            }
        }

        $Hash = Get-FileHash -Algorithm SHA256 -Path $Archive
        Add-Content -Path $Checksums -Value "$($Hash.Hash.ToLowerInvariant())  $(Split-Path -Leaf $Archive)"
    }
}
finally {
    Pop-Location
    Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
}

Write-Host "Wrote release artifacts to $DistDir"
Write-Host "Wrote checksums to $Checksums"
