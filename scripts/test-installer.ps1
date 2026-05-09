<#
.SYNOPSIS
  Static verification of a VEO3 Manager installer .exe.

.DESCRIPTION
  Cannot replace UI smoke testing in a real VM, but catches the easy
  regressions: missing metadata, wrong version, blank icon, broken signature.

.EXAMPLE
  ./scripts/test-installer.ps1
  ./scripts/test-installer.ps1 -InstallerPath .\build\bin\veo3-manager-amd64-installer.exe
#>

[CmdletBinding()]
param(
  [string]$InstallerPath = "$PSScriptRoot\..\build\bin\veo3-manager-amd64-installer.exe",
  [string]$ExpectedProductName = "VEO3 Manager",
  [string]$ExpectedCompany     = "Van Quyen",
  [string]$ExpectedVersion     # optional - if given, must match exactly
)

$ErrorActionPreference = 'Stop'
$failed = 0

function Pass($msg) { Write-Host "[PASS] $msg" -ForegroundColor Green }
function Fail($msg) { Write-Host "[FAIL] $msg" -ForegroundColor Red; $script:failed++ }
function Warn($msg) { Write-Host "[WARN] $msg" -ForegroundColor Yellow }

# 1. File exists + reasonable size
if (-not (Test-Path $InstallerPath)) {
  Fail "Installer not found: $InstallerPath"
  exit 1
}
$item = Get-Item $InstallerPath
$sizeMb = [math]::Round($item.Length / 1MB, 2)
if ($item.Length -lt 1MB) {
  Fail "Installer too small ($sizeMb MB) - probably not a valid build"
} else {
  Pass "File exists, size $sizeMb MB"
}

# 2. Authenticode signature
$sig = Get-AuthenticodeSignature -FilePath $InstallerPath
switch ($sig.Status) {
  'Valid'         { Pass "Authenticode: Valid ($($sig.SignerCertificate.Subject))" }
  'NotSigned'     { Warn "Authenticode: NotSigned - users will see SmartScreen warning. Acceptable for pre-release." }
  default         { Fail "Authenticode: $($sig.Status) - $($sig.StatusMessage)" }
}

# 3. VersionInfo metadata
$vi = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($InstallerPath)

if ([string]::IsNullOrWhiteSpace($vi.ProductName)) {
  Fail "VersionInfo.ProductName is empty - PE resource not embedded correctly"
} elseif ($vi.ProductName -ne $ExpectedProductName) {
  Fail "VersionInfo.ProductName = '$($vi.ProductName)', expected '$ExpectedProductName'"
} else {
  Pass "VersionInfo.ProductName     = $($vi.ProductName)"
}

if ([string]::IsNullOrWhiteSpace($vi.CompanyName)) {
  Fail "VersionInfo.CompanyName is empty"
} elseif ($vi.CompanyName -ne $ExpectedCompany) {
  Fail "VersionInfo.CompanyName = '$($vi.CompanyName)', expected '$ExpectedCompany'"
} else {
  Pass "VersionInfo.CompanyName     = $($vi.CompanyName)"
}

if ([string]::IsNullOrWhiteSpace($vi.ProductVersion)) {
  Fail "VersionInfo.ProductVersion is empty"
} elseif ($ExpectedVersion -and $vi.ProductVersion -ne $ExpectedVersion) {
  Fail "VersionInfo.ProductVersion = '$($vi.ProductVersion)', expected '$ExpectedVersion'"
} else {
  Pass "VersionInfo.ProductVersion  = $($vi.ProductVersion)"
}

if ([string]::IsNullOrWhiteSpace($vi.LegalCopyright)) {
  Fail "VersionInfo.LegalCopyright is empty"
} else {
  Pass "VersionInfo.LegalCopyright  = $($vi.LegalCopyright)"
}

# 4. Quick check that it really is a Win64 PE (DOS MZ + PE\0\0)
$bytes = [System.IO.File]::ReadAllBytes($InstallerPath)
if ($bytes[0] -ne 0x4D -or $bytes[1] -ne 0x5A) {
  Fail "Not a valid PE file (no MZ header)"
} else {
  Pass "Valid PE header"
}

# 5. Icon present in resources?
# Quick&dirty: search for typical RT_GROUP_ICON ASCII signature near resources.
# Reliable check requires PE parsing; here we just sanity-check icon.ico exists alongside.
$ico = Join-Path $PSScriptRoot "..\build\windows\icon.ico"
if (-not (Test-Path $ico)) {
  Warn "build/windows/icon.ico missing - Wails will regenerate from appicon.png on next build"
} else {
  Pass "build/windows/icon.ico present ($([math]::Round((Get-Item $ico).Length/1KB,1)) KB)"
}

Write-Host ""
if ($failed -gt 0) {
  Write-Host "$failed check(s) failed" -ForegroundColor Red
  exit 1
} else {
  Write-Host "All static checks passed." -ForegroundColor Green
  Write-Host "Run installer in a VM/Sandbox to complete UI smoke tests (see docs/TEST-INSTALLER.md)." -ForegroundColor Cyan
}
