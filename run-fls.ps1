$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$dataDir = Join-Path $scriptDir "test-data"
Remove-Item -Recurse -Force $dataDir -ErrorAction SilentlyContinue

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = Join-Path $scriptDir "fls.exe"
$psi.Arguments = "--port 8080 --data-dir `"$dataDir`""
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.CreateNoWindow = $true

$process = [System.Diagnostics.Process]::Start($psi)
Start-Sleep -Milliseconds 500
$process.StandardInput.WriteLine("admin123")
Start-Sleep -Milliseconds 200
$process.StandardInput.WriteLine("admin123")
Start-Sleep -Milliseconds 200
$process.StandardInput.Close()

$processId = $process.Id
$processId | Out-File (Join-Path $scriptDir "fls-pid.txt") -Force
Start-Sleep -Seconds 2
Write-Host "Fls started with PID: $processId"
