$ErrorActionPreference = "Stop"
$repo = if ([string]::IsNullOrWhiteSpace($env:REM_UPDATE_REPO)) { "crnobog69/rem" } else { $env:REM_UPDATE_REPO }
$ref = if ([string]::IsNullOrWhiteSpace($env:REM_UPDATE_REF)) { "master" } else { $env:REM_UPDATE_REF }

$localScriptPath = $null
if ($MyInvocation -and $MyInvocation.MyCommand -and -not [string]::IsNullOrWhiteSpace($MyInvocation.MyCommand.Path)) {
  $localScriptPath = $MyInvocation.MyCommand.Path
}

if (-not [string]::IsNullOrWhiteSpace($localScriptPath)) {
  $scriptDir = Split-Path -Parent $localScriptPath
  $localInstall = Join-Path $scriptDir "install.ps1"
  if (Test-Path -LiteralPath $localInstall) {
    & $localInstall
    exit 0
  }
}

$url = "https://raw.githubusercontent.com/$repo/$ref/scripts/install.ps1"
Invoke-Expression ((Invoke-WebRequest -Uri $url -UseBasicParsing).Content)
