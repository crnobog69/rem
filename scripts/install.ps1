$ErrorActionPreference = "Stop"

$repo = if ([string]::IsNullOrWhiteSpace($env:REM_UPDATE_REPO)) { "crnobog69/rem" } else { $env:REM_UPDATE_REPO }

$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { throw "Unsupported architecture" }
$asset = "rem-windows-$arch.exe"
$url = "https://github.com/$repo/releases/latest/download/$asset"

$destDir = Join-Path $env:USERPROFILE "bin"
New-Item -ItemType Directory -Path $destDir -Force | Out-Null
$dest = Join-Path $destDir "rem.exe"

Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
Write-Host "Installed rem to $dest"
