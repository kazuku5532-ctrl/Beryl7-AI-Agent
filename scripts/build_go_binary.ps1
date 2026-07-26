$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host " Building Beryl 7 Native Go Daemon (arm64)... " -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

$outputDir = "bin"
if (!(Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

$binaryPath = "$outputDir\beryl7-agent"

Push-Location "go-agent"
try {
    Write-Host "Running go build with -ldflags='-s -w'..." -ForegroundColor Yellow
    $env:GOOS = "linux"
    $env:GOARCH = "arm64"
    $env:CGO_ENABLED = "0"
    & "C:\Program Files\Go\bin\go.exe" build -ldflags="-s -w" -o "..\$binaryPath" ./cmd
    Write-Host "Go build completed successfully!" -ForegroundColor Green
} finally {
    Pop-Location
}

if (Test-Path $binaryPath) {
    $fileSize = (Get-Item $binaryPath).Length
    $fileSizeMB = [math]::Round($fileSize / 1MB, 2)
    Write-Host "Binary Output: $binaryPath" -ForegroundColor White
    Write-Host "Binary Size: $fileSizeMB MB" -ForegroundColor White

    if ($fileSize -gt 10MB) {
        Write-Host "WARNING: Binary size exceeds 10MB threshold!" -ForegroundColor Red
        exit 1
    } else {
        Write-Host "VERIFICATION PASSED: Binary size is under 10MB threshold!" -ForegroundColor Green
    }
} else {
    Write-Host "ERROR: Output binary not found!" -ForegroundColor Red
    exit 1
}
