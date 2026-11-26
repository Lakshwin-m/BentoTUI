param(
    [string]$Version = "dev",
    [switch]$SkipTests,
    [switch]$NoInstall
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    if (-not $SkipTests) {
        Write-Host "==> go test ./..." -ForegroundColor Cyan
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw "tests failed" }
    }

    New-Item -ItemType Directory -Force -Path "$root\dist" | Out-Null
    Write-Host "==> go build" -ForegroundColor Cyan
    go build -trimpath -ldflags "-s -w -X main.version=$Version" -o "$root\dist\bento.exe" ./cmd/bento
    if ($LASTEXITCODE -ne 0) { throw "build failed" }

    if ($NoInstall) {
        Write-Host "Built $root\dist\bento.exe" -ForegroundColor Green
        return
    }

    $installDir = Join-Path $env:LOCALAPPDATA "bento"
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Copy-Item "$root\dist\bento.exe" (Join-Path $installDir "bento.exe") -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @($userPath -split ";" | Where-Object { $_ -ne "" })
    if ($entries -notcontains $installDir) {
        $newPath = (($entries + $installDir) -join ";")
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "Added $installDir to your user PATH (new terminal tabs will see it)." -ForegroundColor Yellow
    }
    Write-Host "Installed $(Join-Path $installDir 'bento.exe')" -ForegroundColor Green
}
finally {
    Pop-Location
}
