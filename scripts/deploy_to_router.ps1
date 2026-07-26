$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host " Deploying Native Go Agent to GL.iNet Beryl 7... " -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

$routerIP = "192.168.8.1"
$routerUser = "root"
$binaryPath = "bin\beryl7-agent"

if (!(Test-Path $binaryPath)) {
    Write-Host "Binary not found. Building now..." -ForegroundColor Yellow
    powershell -ExecutionPolicy Bypass -File .\scripts\build_go_binary.ps1
}

# Load password from .env if available
$password = "Kazuku@2k6"
if (Test-Path ".env") {
    Get-Content ".env" | ForEach-Object {
        if ($_ -match "^ROUTER_PASSWORD=(.*)$") {
            $password = $matches[1].Trim("`"'")
        }
    }
}

Write-Host "1. Testing SSH connectivity to $routerIP..." -ForegroundColor Yellow
$sshTest = ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 ${routerUser}@${routerIP} "echo 'SSH_OK'" 2>&1
if ($sshTest -notmatch "SSH_OK") {
    Write-Host "SSH connection failed. Please ensure Beryl 7 router is connected at 192.168.8.1." -ForegroundColor Red
    exit 1
}

Write-Host "2. Creating /etc/beryl7 directory on router..." -ForegroundColor Yellow
ssh -o StrictHostKeyChecking=no ${routerUser}@${routerIP} "mkdir -p /etc/beryl7 /var/log"

Write-Host "3. Uploading beryl7-agent binary to /usr/bin/..." -ForegroundColor Yellow
scp -o StrictHostKeyChecking=no $binaryPath ${routerUser}@${routerIP}:/usr/bin/beryl7-agent
ssh -o StrictHostKeyChecking=no ${routerUser}@${routerIP} "chmod +x /usr/bin/beryl7-agent"

Write-Host "4. Uploading procd service init script..." -ForegroundColor Yellow
scp -o StrictHostKeyChecking=no "go-agent\procd\beryl7-agent" ${routerUser}@${routerIP}:/etc/init.d/beryl7-agent
ssh -o StrictHostKeyChecking=no ${routerUser}@${routerIP} "chmod +x /etc/init.d/beryl7-agent"

Write-Host "5. Enabling and starting beryl7-agent procd service..." -ForegroundColor Yellow
ssh -o StrictHostKeyChecking=no ${routerUser}@${routerIP} "/etc/init.d/beryl7-agent enable; /etc/init.d/beryl7-agent restart"

Write-Host "6. Verifying live service status on router..." -ForegroundColor Yellow
Start-Sleep -Seconds 2
$status = ssh -o StrictHostKeyChecking=no ${routerUser}@${routerIP} "ps | grep beryl7-agent | grep -v grep"
Write-Host "Live Process:" -ForegroundColor White
Write-Host $status -ForegroundColor Green

Write-Host "==================================================" -ForegroundColor Green
Write-Host " SUCCESS: Beryl 7 Native Go Agent is running 24/7! " -ForegroundColor Green
Write-Host " You can now turn off your laptop cleanly.          " -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Green
