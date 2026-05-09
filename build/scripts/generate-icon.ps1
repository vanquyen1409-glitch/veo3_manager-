param(
  [int]$Size = 1024,
  [string]$OutPng = "$PSScriptRoot\..\appicon.png"
)

# Generates a flat, modern "video/play" app icon for VEO3 Manager.
# Pure .NET System.Drawing - no external deps. Output: 1024x1024 PNG.

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$bmp = New-Object System.Drawing.Bitmap($Size, $Size)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
$g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
$g.Clear([System.Drawing.Color]::Transparent)

# 1) Rounded-square background with vertical indigo->violet gradient
$pad = [int]($Size * 0.06)
$rectF = New-Object System.Drawing.RectangleF($pad, $pad, ($Size - 2 * $pad), ($Size - 2 * $pad))
$radius = [single]($Size * 0.22)

function New-RoundedPath {
  param([System.Drawing.RectangleF]$R, [single]$Rad)
  $p = New-Object System.Drawing.Drawing2D.GraphicsPath
  $d = $Rad * 2
  $p.AddArc($R.X, $R.Y, $d, $d, 180, 90)
  $p.AddArc(($R.Right - $d), $R.Y, $d, $d, 270, 90)
  $p.AddArc(($R.Right - $d), ($R.Bottom - $d), $d, $d, 0, 90)
  $p.AddArc($R.X, ($R.Bottom - $d), $d, $d, 90, 90)
  $p.CloseFigure()
  return $p
}

$bgPath = New-RoundedPath $rectF $radius

# Indigo-600 (#4F46E5) -> Violet-600 (#7C3AED) diagonal gradient
$grad = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
  $rectF,
  [System.Drawing.Color]::FromArgb(255, 79, 70, 229),
  [System.Drawing.Color]::FromArgb(255, 124, 58, 237),
  45.0)

$g.FillPath($grad, $bgPath)

# 2) Subtle inner highlight (top edge) for "modern flat" depth without skeuomorphism
$inner = New-Object System.Drawing.RectangleF(($rectF.X + 8), ($rectF.Y + 8), ($rectF.Width - 16), ($rectF.Height * 0.4))
$hi = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
  $inner,
  [System.Drawing.Color]::FromArgb(38, 255, 255, 255),
  [System.Drawing.Color]::FromArgb(0, 255, 255, 255),
  90.0)
$innerPath = New-RoundedPath $inner ($radius - 8)
$g.FillPath($hi, $innerPath)

# 3) White play triangle, optically centered (geometric centroid != visual center)
$cx = $Size / 2.0
$cy = $Size / 2.0
$triR = $Size * 0.22

# Equilateral-ish play triangle pointing right; offset slightly right so it looks centered
$ox = $Size * 0.025
$pts = @(
  (New-Object System.Drawing.PointF(($cx - $triR * 0.55 + $ox), ($cy - $triR))),
  (New-Object System.Drawing.PointF(($cx - $triR * 0.55 + $ox), ($cy + $triR))),
  (New-Object System.Drawing.PointF(($cx + $triR * 1.05 + $ox), $cy))
)

$tri = New-Object System.Drawing.Drawing2D.GraphicsPath
$tri.AddPolygon($pts)

# Soft drop shadow under triangle
$shadowBmp = New-Object System.Drawing.Bitmap($Size, $Size)
$sg = [System.Drawing.Graphics]::FromImage($shadowBmp)
$sg.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$shadowBrush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(80, 0, 0, 0))
$shadowMatrix = New-Object System.Drawing.Drawing2D.Matrix
$shadowMatrix.Translate(0, $Size * 0.012)
$shadowTri = $tri.Clone()
$shadowTri.Transform($shadowMatrix)
$sg.FillPath($shadowBrush, $shadowTri)
$sg.Dispose()
$g.DrawImage($shadowBmp, 0, 0)
$shadowBmp.Dispose()

$white = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::White)
$g.FillPath($white, $tri)

# 4) Two small "film-strip" notches on left & right edges for video association
$notchW = [int]($Size * 0.04)
$notchH = [int]($Size * 0.06)
$notchGap = [int]($Size * 0.10)
$notchBrush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(120, 255, 255, 255))
for ($i = -1; $i -le 1; $i++) {
  $y = $cy - ($notchH / 2) + ($i * $notchGap)
  # left notches
  $g.FillRectangle($notchBrush, ($pad + 8), $y, $notchW, $notchH)
  # right notches
  $g.FillRectangle($notchBrush, ($Size - $pad - 8 - $notchW), $y, $notchW, $notchH)
}

$g.Dispose()

$outDir = Split-Path -Parent $OutPng
if (-not (Test-Path $outDir)) { New-Item -ItemType Directory -Force -Path $outDir | Out-Null }
$bmp.Save($OutPng, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()

Write-Host "Wrote $OutPng ($Size x $Size)"
