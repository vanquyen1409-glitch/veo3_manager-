param(
  [string]$Version = "0.1.0",
  [switch]$NoUpx,
  [switch]$NoInstaller,
  [ValidateSet('amd64','arm64','both')]
  [string]$Arch = 'amd64'
)

# Production Windows build for VEO3 Manager.
#
# Flags:
#   -Version       Sets main.version via -ldflags. Stamped into the binary.
#   -NoUpx         Skip UPX compression even if upx.exe is on PATH.
#   -NoInstaller   Skip the NSIS Setup.exe step.
#   -Arch          amd64 (default), arm64, or both. "both" produces a
#                  combined NSIS installer that picks the right binary at install time.

$ErrorActionPreference = 'Stop'

$root = Resolve-Path (Join-Path $PSScriptRoot '..\..')
Set-Location $root

$buildDate = (Get-Date).ToUniversalTime().ToString('o')

# -ldflags meaning:
#   -s  : strip the symbol table       (smaller binary, no symbol info for debuggers)
#   -w  : strip DWARF debug info       (smaller binary, no source-line info on panic)
#   -X  : inject a string at build time into a package var (here: main.version, main.buildDate)
$ldflags = "-s -w -X main.version=$Version -X main.buildDate=$buildDate"

# Reusable arg list passed to wails build:
#   -platform <os/arch>  : target triple
#   -ldflags <string>    : forwarded to `go build -ldflags`
#   -trimpath            : remove absolute filesystem paths from the binary -> smaller & reproducible
#   -clean               : wipe build/bin before rebuilding to avoid stale artifacts
#   -webview2 embed      : bundle the Edge WebView2 bootstrap installer inside the .exe;
#                          if the user lacks WebView2 runtime, the app installs it on first run.
#                          Trade-off: bigger .exe (~2 MB), but works offline / on fresh Windows.
#   -nsis                : also produce a Setup-style installer (.exe) using NSIS.
#                          Requires makensis on PATH (already installed in this env).
function Invoke-WailsBuild {
  param([string]$Platform, [bool]$WithNsis)

  Write-Host "`n=== wails build [$Platform] ===" -ForegroundColor Cyan

  $args = @(
    'build',
    '-platform', $Platform,
    '-ldflags', $ldflags,
    '-trimpath',
    '-clean',
    '-webview2', 'embed'
  )
  if ($WithNsis) { $args += '-nsis' }

  & wails @args
  if ($LASTEXITCODE -ne 0) { throw "wails build failed for $Platform (exit $LASTEXITCODE)" }
}

function Show-Size {
  param([string]$Path, [string]$Label)
  if (Test-Path $Path) {
    $sizeMb = [math]::Round((Get-Item $Path).Length / 1MB, 2)
    Write-Host ("  {0,-60} {1,8} MB" -f $Label, $sizeMb) -ForegroundColor Green
  }
}

function Compress-WithUpx {
  param([string]$ExePath)
  if ($NoUpx) { Write-Host "  UPX: skipped (-NoUpx)" -ForegroundColor Yellow; return }
  $upx = Get-Command upx -ErrorAction SilentlyContinue
  if (-not $upx) { Write-Host "  UPX: not installed - skipping (winget install upx)" -ForegroundColor Yellow; return }

  $before = (Get-Item $ExePath).Length
  Write-Host "  UPX: compressing $ExePath" -ForegroundColor Cyan
  # --compress-resources=0 preserves the PE resource directory uncompressed so
  # Windows still finds the VERSIONINFO block and the embedded app icon. Without
  # this flag UPX --lzma packs resources into the compressed payload and the
  # exe's "Properties -> Details" tab + taskbar icon go blank.
  & upx --best --lzma --compress-resources=0 --no-progress $ExePath
  if ($LASTEXITCODE -ne 0) {
    Write-Host "  UPX: failed (exit $LASTEXITCODE) - leaving original binary" -ForegroundColor Yellow
    return
  }
  $after = (Get-Item $ExePath).Length
  $saved = [math]::Round((1 - ($after / $before)) * 100, 1)
  Write-Host ("  UPX: {0:N0} -> {1:N0} bytes (saved {2}%)" -f $before, $after, $saved) -ForegroundColor Green
}

Write-Host "Building VEO3 Manager v$Version (Windows / $Arch)" -ForegroundColor Cyan
Write-Host "Build date: $buildDate" -ForegroundColor DarkGray

$buildInstaller = -not $NoInstaller

switch ($Arch) {
  'amd64' { Invoke-WailsBuild -Platform 'windows/amd64' -WithNsis $buildInstaller }
  'arm64' { Invoke-WailsBuild -Platform 'windows/arm64' -WithNsis $buildInstaller }
  'both'  {
    # When building both, only run -nsis on the second pass: Wails will pick up
    # both binaries (renamed to <name>-amd64.exe / <name>-arm64.exe) and produce
    # one universal installer.
    Invoke-WailsBuild -Platform 'windows/amd64' -WithNsis $false
    Invoke-WailsBuild -Platform 'windows/arm64' -WithNsis $buildInstaller
  }
}

# UPX-compress the produced .exe(s)
$bin = Join-Path $root 'build\bin'
Get-ChildItem -Path $bin -Filter '*.exe' -File | Where-Object { $_.Name -notlike '*installer*' } | ForEach-Object {
  Compress-WithUpx -ExePath $_.FullName
}

Write-Host "`nArtifacts:" -ForegroundColor Cyan
Get-ChildItem -Path $bin -Filter '*.exe' -File | Sort-Object Name | ForEach-Object {
  Show-Size -Path $_.FullName -Label $_.Name
}

Write-Host "`nDone. Output dir: $bin" -ForegroundColor Green
