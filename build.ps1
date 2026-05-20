param(
    [string]$OutputDir = "./dist"
)

$ErrorActionPreference = "Stop"
$Version = if (git describe --tags --always 2>$null) { git describe --tags --always } else { "dev" }
$Date = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
$Commit = if (git rev-parse --short HEAD 2>$null) { git rev-parse --short HEAD } else { "unknown" }
$LDFlags = "-s -w -X main.version=$Version -X main.commit=$Commit -X main.date=$Date"

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
Write-Host "Building fls Windows amd64..." -ForegroundColor Green
mise.exe x '--' go build -ldflags "$LDFlags" -o "$OutputDir/fls.exe" .
Write-Host "Build complete: $OutputDir/fls.exe" -ForegroundColor Green
