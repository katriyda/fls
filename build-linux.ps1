$OutputDir = "./dist"
$Version = "dev"
try {
    $gitVer = git describe --tags --always 2>$null
    if ($gitVer) { $Version = $gitVer }
} catch {}
$Date = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
$Commit = "unknown"
try {
    $gitCommit = git rev-parse --short HEAD 2>$null
    if ($gitCommit) { $Commit = $gitCommit }
} catch {}
$LDFlags = "-s -w -X main.version=$Version -X main.commit=$Commit -X main.date=$Date"

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
Write-Host "Building fls Linux amd64..." -ForegroundColor Green
$env:GOOS = "linux"
$env:GOARCH = "amd64"
mise.exe x '--' go build -ldflags "$LDFlags" -o "$OutputDir/fls-linux-amd64" .
Write-Host "Build complete: $OutputDir/fls-linux-amd64" -ForegroundColor Green
