param(
    [ValidateSet("build", "test", "vet", "lint", "run", "install")]
    [string]$Task = "build"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $root
try {
    switch ($Task) {
        "build" {
            go build -o "$root\bento.exe" ./cmd/bento
        }
        "test" {
            go test ./...
        }
        "vet" {
            go vet ./...
        }
        "lint" {
            $staticcheck = Get-Command staticcheck -ErrorAction SilentlyContinue
            if ($staticcheck) { staticcheck ./... } else { Write-Host "staticcheck not installed; running go vet instead" -ForegroundColor Yellow; go vet ./... }
        }
        "run" {
            go run ./cmd/bento
        }
        "install" {
            & "$root\scripts\build.ps1"
        }
    }
}
finally {
    Pop-Location
}
