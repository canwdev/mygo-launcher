<# .SYNOPSIS
    Regenerates the runtime icon parts from resources/icon.svg:
    icon-base.png (background), icon-accent.png (accent mask, with the
    16% arrow kept translucent) and icon-gear.png (the settings badge).
    Requires Chrome or Edge, as generate-icon.ps1 does.
#>

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
Add-Type -AssemblyName System.Drawing

function Find-Browser {
    $candidates = @(
        "$env:ProgramFiles\Google\Chrome\Application\chrome.exe",
        "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe",
        "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe",
        "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
    )
    foreach ($c in $candidates) { if (Test-Path $c) { return $c } }
    throw 'no browser'
}
$browser = Find-Browser
$tmpDir = Join-Path (Get-Location) 'resources\.render'
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

function Render-Svg([string]$svgText, [int]$size, [string]$outPng) {
    $g = [guid]::NewGuid().ToString('N')
    $htmlPath = Join-Path $tmpDir ("r_" + $g + ".html")
    $renderTmp = Join-Path $tmpDir ("r_" + $g + ".png")
    $keyedTmp = Join-Path $tmpDir ("k_" + $g + ".png")
    $html = "<!DOCTYPE html><html><head><meta charset=`"utf-8`"><style>html,body{margin:0;padding:0;background:#FF00FF;overflow:hidden}</style></head><body style='margin:0;padding:0'>$svgText</body></html>"
    [System.IO.File]::WriteAllText($htmlPath, $html, (New-Object System.Text.UTF8Encoding($false)))
    & $browser --headless --disable-gpu --screenshot="$($renderTmp.Replace('\','/'))" --window-size=$size,$size --force-device-scale-factor=1 --virtual-time-budget=3000 "file:///$($htmlPath.Replace('\','/'))"
    $seen = $false
    for ($i = 0; $i -lt 30; $i++) {
        if (Test-Path $renderTmp) { $seen = $true; break }
        Start-Sleep -Milliseconds 500
    }
    Remove-Item $htmlPath -Force -ErrorAction SilentlyContinue
    if (-not $seen) { throw "render failed: $outPng" }

    $srcBmp = [System.Drawing.Bitmap]::FromFile((Resolve-Path $renderTmp))
    $bmp = New-Object System.Drawing.Bitmap($srcBmp.Width, $srcBmp.Height, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $gg = [System.Drawing.Graphics]::FromImage($bmp)
    $gg.DrawImage($srcBmp, 0, 0, $srcBmp.Width, $srcBmp.Height)
    $gg.Dispose()
    $srcBmp.Dispose()
    $rc = New-Object System.Drawing.Rectangle(0, 0, $bmp.Width, $bmp.Height)
    $data = $bmp.LockBits($rc, [System.Drawing.Imaging.ImageLockMode]::ReadWrite, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $stride = $data.Stride
    $bytes = New-Object byte[] ($stride * $bmp.Height)
    [System.Runtime.InteropServices.Marshal]::Copy($data.Scan0, $bytes, 0, $bytes.Length)
    for ($y = 0; $y -lt $bmp.Height; $y++) {
        $row = $y * $stride
        for ($x = 0; $x -lt $bmp.Width; $x++) {
            $i = $row + $x * 4
            $B = $bytes[$i]; $G = $bytes[$i + 1]; $R = $bytes[$i + 2]
            if ($R -gt 248 -and $B -gt 248) {
                $a = $G
                if ($a -gt 255) { $a = 255 }
                $bytes[$i] = 255; $bytes[$i + 1] = 255; $bytes[$i + 2] = 255; $bytes[$i + 3] = $a
            } else {
                $bytes[$i + 3] = 255
            }
        }
    }
    [System.Runtime.InteropServices.Marshal]::Copy($bytes, 0, $data.Scan0, $bytes.Length)
    $bmp.UnlockBits($data)
    $bmp.Save($keyedTmp, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
    Copy-Item -Path $keyedTmp -Destination $outPng -Force
    Remove-Item $renderTmp, $keyedTmp -Force -ErrorAction SilentlyContinue
}

$iconSvg = Get-Content 'resources\icon.svg' -Raw
$baseSvg = @'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <rect x="1.5" y="1.5" width="509" height="509" rx="112" fill="#ffffff"/>
  <rect x="1.5" y="1.5" width="509" height="509" rx="112" fill="none" stroke="#e6e8ea" stroke-width="3"/>
</svg>
'@
$accentSvg = $iconSvg -replace '(?m)^\s*<rect[^>]*fill="#ffffff"[^>]*/>\s*$', '' -replace '(?m)^\s*<rect[^>]*fill="none"[^>]*/>\s*$', '' -replace 'fill="#00abf2"', 'fill="#ffffff"'
$gearSvg = @'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" width="256" height="256">
  <circle cx="128" cy="128" r="118" fill="#ffffff"/>
  <g transform="translate(52 52) scale(6.3333)" fill="none" stroke="#1c1c1f" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
    <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
    <circle cx="12" cy="12" r="3"/>
  </g>
</svg>
'@

Render-Svg $baseSvg 512 'resources\icon-base.png'
Render-Svg $accentSvg 512 'resources\icon-accent.png'
Render-Svg $gearSvg 256 'resources\icon-gear.png'
"done"
