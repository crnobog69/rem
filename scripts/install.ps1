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

function Test-PathContainsEntry([string]$pathValue, [string]$entry) {
  if ([string]::IsNullOrWhiteSpace($pathValue)) {
    return $false
  }
  $needle = $entry -replace '[\\]+$', ''
  foreach ($part in ($pathValue -split ";")) {
    $candidate = $part.Trim()
    if ([string]::IsNullOrWhiteSpace($candidate)) {
      continue
    }
    $candidate = $candidate -replace '[\\]+$', ''
    if ([string]::Equals($candidate, $needle, [StringComparison]::OrdinalIgnoreCase)) {
      return $true
    }
  }
  return $false
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$addedToUserPath = $false
if (-not (Test-PathContainsEntry $userPath $destDir)) {
  $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $destDir } else { "$userPath;$destDir" }
  [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
  $addedToUserPath = $true
}

if (-not (Test-PathContainsEntry $env:Path $destDir)) {
  $env:Path = if ([string]::IsNullOrWhiteSpace($env:Path)) { $destDir } else { "$env:Path;$destDir" }
}

if ($addedToUserPath) {
  Write-Host "Added $destDir to user PATH."
}
Write-Host "You can run: rem -v"
