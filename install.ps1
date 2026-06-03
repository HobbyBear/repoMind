# ============================================================
# RepoMind Windows 一键安装脚本
# Usage: powershell -ExecutionPolicy Bypass -File install.ps1
#   或: iex (iwr -useb https://raw.githubusercontent.com/HobbyBear/repoMind/master/install.ps1)
#
# 脚本会自动:
#   1. 检测 Windows 平台（amd64 或 arm64）
#   2. 从 GitHub Releases 下载对应架构的 repomind.exe
#   3. 安装到 C:\ProgramData\repomind\bin 或 %LOCALAPPDATA%\repomind\bin
#   4. 自动添加到 系统 PATH 或 用户 PATH
#   5. 验证安装成功
# ============================================================

$ErrorActionPreference = "Stop"
$RepoUrl = "https://github.com/HobbyBear/repoMind/releases/latest/download"

Write-Host "RepoMind Windows Installer" -ForegroundColor Cyan
Write-Host "==========================" -ForegroundColor Cyan

# --- Detect architecture ---
$arch = $env:PROCESSOR_ARCHITECTURE
$is64 = $env:PROCESSOR_ARCHITEW6432
$isArm = $false

if ($arch -eq "ARM64" -or $is64 -eq "ARM64") {
    $isArm = $true
}

if ($isArm) {
    $binaryName = "repomind-windows-arm64.exe"
    Write-Host "Detected: Windows ARM64" -ForegroundColor Green
} else {
    $binaryName = "repomind-windows-amd64.exe"
    Write-Host "Detected: Windows AMD64" -ForegroundColor Green
}

$downloadUrl = "$RepoUrl/$binaryName"

# --- Check for admin rights ---
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

# --- Choose install directory ---
if ($isAdmin) {
    $installDir = "C:\ProgramData\repomind\bin"
    $targetPath = [Environment]::GetFolderPath("CommonApplicationData") + "\repomind\bin"
} else {
    $installDir = "$env:LOCALAPPDATA\repomind\bin"
    $targetPath = $installDir
}
$exePath = "$targetPath\repomind.exe"

# --- Download ---
Write-Host "Downloading repomind.exe..." -ForegroundColor Yellow
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $null = New-Item -ItemType Directory -Force -Path $targetPath
    Invoke-WebRequest -Uri $downloadUrl -OutFile "$targetPath\repomind.exe.tmp" -UseBasicParsing
    Move-Item -Force "$targetPath\repomind.exe.tmp" "$targetPath\repomind.exe"
    Write-Host "Downloaded: $exePath" -ForegroundColor Green
} catch {
    Write-Host "Download failed: $_" -ForegroundColor Red
    exit 1
}

# --- Add to PATH ---
$scope = if ($isAdmin) { "Machine" } else { "User" }
$currentPath = [Environment]::GetEnvironmentVariable("PATH", $scope)
$currentParts = $currentPath -split ";"

if ($currentParts -notcontains $targetPath) {
    $newPath = "$currentPath;$targetPath"
    [Environment]::SetEnvironmentVariable("PATH", $newPath, $scope)
    Write-Host "Added to $scope PATH: $targetPath" -ForegroundColor Green
    # Also update current session PATH
    $env:PATH = "$env:PATH;$targetPath"
} else {
    Write-Host "$targetPath is already in $scope PATH" -ForegroundColor Green
}

# --- Verify ---
Write-Host ""
try {
    $version = & "$exePath" --help 2>&1 | Select-Object -First 1
    Write-Host "Verified: $version" -ForegroundColor Green
} catch {
    Write-Host "Warning: could not verify installation: $_" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "RepoMind installed successfully!" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Open a NEW PowerShell / cmd window (to pick up PATH changes)"
Write-Host "  2. cd into your project"
Write-Host "  3. Run: repomind install"
Write-Host ""
