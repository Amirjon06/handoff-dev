$ErrorActionPreference = "Stop"

$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")
$DistDir = Join-Path $RootDir "dist"

New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

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
        $env:CGO_ENABLED = "0"
        $env:GOOS = $Target.GOOS
        $env:GOARCH = $Target.GOARCH

        $Output = Join-Path $DistDir "relay-$($Target.GOOS)-$($Target.GOARCH)$($Target.Extension)"
        Write-Host "Building $Output"
        go build -trimpath -ldflags "-s -w" -o $Output ./cmd/relay
    }
}
finally {
    Pop-Location
    Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
}
