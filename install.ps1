$ErrorActionPreference = "Stop"

$Repo = if ($env:REPO) { $env:REPO } else { "ai4next/superman" }
$Version = if ($env:VERSION) { $env:VERSION } else { "latest" }
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\superman" }
$BinName = if ($env:BIN_NAME) { $env:BIN_NAME } else { "sm.exe" }

if ($Version -eq "latest") {
    $response = Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest"
    $finalUrl = $response.BaseResponse.ResponseUri.AbsoluteUri
    $Version = Split-Path $finalUrl -Leaf
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    throw "Could not resolve latest release version."
}

$asset = "sm-$Version-windows-amd64.exe"
$url = "https://github.com/$Repo/releases/download/$Version/$asset"
$target = Join-Path $InstallDir $BinName

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Write-Host "Downloading $url"
Invoke-WebRequest -Uri $url -OutFile $target

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$pathParts = @()
if ($userPath) {
    $pathParts = $userPath -split ';' | Where-Object { $_ }
}

if ($pathParts -notcontains $InstallDir) {
    $newPath = ($pathParts + $InstallDir) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "Added $InstallDir to your user PATH. Open a new terminal before running sm."
}

Write-Host "Installed sm $Version to $target"
Write-Host "Run: sm --help"
