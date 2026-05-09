<#
.SYNOPSIS
  Bump VEO3 Manager version (SemVer) across wails.json + frontend/package.json.

.DESCRIPTION
  Reads the current productVersion from wails.json, increments the requested
  part (major/minor/patch) per SemVer rules, then writes the new value back
  to BOTH wails.json (info.productVersion) and frontend/package.json (version).

  Uses targeted regex replacement so JSON formatting (key order, indentation,
  trailing newline) is preserved exactly.

.PARAMETER Part
  Which SemVer part to bump: 'major', 'minor', or 'patch'.

.PARAMETER DryRun
  Show what would change without writing files.

.EXAMPLE
  ./scripts/bump-version.ps1 patch        # 0.1.0 -> 0.1.1
  ./scripts/bump-version.ps1 minor        # 0.1.1 -> 0.2.0
  ./scripts/bump-version.ps1 major        # 0.2.0 -> 1.0.0
  ./scripts/bump-version.ps1 patch -DryRun
#>

[CmdletBinding()]
param(
  [Parameter(Mandatory = $true, Position = 0)]
  [ValidateSet('major', 'minor', 'patch')]
  [string]$Part,

  [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

# Resolve project root regardless of where the script is invoked from.
$root = Resolve-Path (Join-Path $PSScriptRoot '..')
$wailsJson = Join-Path $root 'wails.json'
$pkgJson   = Join-Path $root 'frontend\package.json'

if (-not (Test-Path $wailsJson)) { throw "wails.json not found at $wailsJson" }
if (-not (Test-Path $pkgJson))   { throw "frontend/package.json not found at $pkgJson" }

# --- Read current version from wails.json -----------------------------------
# Match the productVersion line specifically (the file also has author.email,
# companyName, etc., so we anchor on the field name to avoid false matches).
$wailsContent = Get-Content $wailsJson -Raw -Encoding UTF8
$wailsRe = '("productVersion"\s*:\s*")(\d+)\.(\d+)\.(\d+)(")'
$m = [regex]::Match($wailsContent, $wailsRe)
if (-not $m.Success) {
  throw "Could not find a SemVer productVersion (X.Y.Z) in wails.json. Is info.productVersion set?"
}

$major = [int]$m.Groups[2].Value
$minor = [int]$m.Groups[3].Value
$patch = [int]$m.Groups[4].Value
$current = "$major.$minor.$patch"

# --- Bump per SemVer --------------------------------------------------------
switch ($Part) {
  'major' { $major++; $minor = 0; $patch = 0 }
  'minor' { $minor++; $patch = 0 }
  'patch' { $patch++ }
}
$next = "$major.$minor.$patch"

Write-Host "Current version : $current"
Write-Host "Bump            : $Part"
Write-Host "New version     : $next"

if ($DryRun) {
  Write-Host "`nDry run - no files written." -ForegroundColor Yellow
  return
}

# --- Update wails.json ------------------------------------------------------
# Regex callback rebuilds the matched line with the new version, preserving
# original quoting/spacing.
$newWails = [regex]::Replace($wailsContent, $wailsRe, {
  param($m)
  "$($m.Groups[1].Value)$next$($m.Groups[5].Value)"
})

# --- Update frontend/package.json ------------------------------------------
# package.json has top-level "version": "X.Y.Z". Anchor on a fresh line to
# avoid hitting nested deps that happen to use SemVer values.
$pkgContent = Get-Content $pkgJson -Raw -Encoding UTF8
$pkgRe = '(?m)^(\s*"version"\s*:\s*")(\d+\.\d+\.\d+(?:-[\w\.\-]+)?)(",?\s*)$'
if (-not [regex]::IsMatch($pkgContent, $pkgRe)) {
  throw "Could not find top-level `"version`" in frontend/package.json"
}
$newPkg = [regex]::Replace($pkgContent, $pkgRe, {
  param($m)
  "$($m.Groups[1].Value)$next$($m.Groups[3].Value)"
})

# --- Write back (UTF-8, no BOM, preserve line endings) ----------------------
function Write-Text {
  param([string]$Path, [string]$Content)
  # Detect existing line ending so we don't accidentally convert CRLF<->LF.
  $orig = [System.IO.File]::ReadAllText($Path)
  if ($orig.Contains("`r`n") -and -not $Content.Contains("`r`n")) {
    $Content = $Content -replace "`n", "`r`n"
  }
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($Path, $Content, $utf8NoBom)
}

Write-Text -Path $wailsJson -Content $newWails
Write-Text -Path $pkgJson   -Content $newPkg

Write-Host "`nUpdated:"
Write-Host "  wails.json            -> info.productVersion = $next"
Write-Host "  frontend/package.json -> version             = $next"
Write-Host "`nNew version: $next" -ForegroundColor Green
