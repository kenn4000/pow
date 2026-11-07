Add-Type -AssemblyName System.Drawing

$folder = "C:\Users\kenn\Documents\prisonersofwar\backend\internal\skins\icons\herocart"
$outputDir = "C:\Users\kenn\Documents\prisonersofwar\output"

if (-Not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

# 1. THE EXPLICIT ORDER
# Put the exact filenames here in the exact order you want them to play in-game.
$orderedFilenames = @(
    "cart_closed.png",
    "cart_open0.png",
    "cart_open1.png",
    "cart_open2.png",
    "cart_open3.png"
)

# Verify all files exist
$pngFiles = @()
foreach ($name in $orderedFilenames) {
    $path = Join-Path $folder $name
    if (Test-Path $path) {
        $pngFiles += Get-Item $path
    } else {
        Write-Host "WARNING: Could not find $name in folder!" -ForegroundColor Red
    }
}

if ($pngFiles.Count -eq 0) {
    Write-Host "ERROR: No matching images found." -ForegroundColor Red
    exit
}

$targetHeight = 512
$buffer = 64
$colCount = 3 # 3 columns x 2 rows for 5 images

$safeName = "herocart_anim"
$tempScaledDir = Join-Path $folder "_scaled_temp"

if (Test-Path $tempScaledDir) { Remove-Item -Path $tempScaledDir -Recurse -Force }
New-Item -ItemType Directory -Path $tempScaledDir | Out-Null

$maxScaledWidth = 0
$maxScaledHeight = 0

# 2. Resize each image proportionally to fit within the target height
foreach ($file in $pngFiles) {
    $destFile = Join-Path $tempScaledDir $file.Name

    & magick "$($file.FullName)" -background none -resize "x$targetHeight" "$destFile"

    $dims = & magick identify -format "%h %w" "$destFile"
    $parts = $dims -split ' '
    $h = [int]$parts[0]
    $w = [int]$parts[1]

    if ($w -gt $maxScaledWidth) { $maxScaledWidth = $w }
    if ($h -gt $maxScaledHeight) { $maxScaledHeight = $h }
}

$frameWidth = $maxScaledWidth + ($buffer * 2)
$frameHeight = $maxScaledHeight + ($buffer * 2)

Write-Host "Calculated Frame Size: $($frameWidth)x$($frameHeight)" -ForegroundColor Yellow

# 3. Extent every image to the exact same frame size with the buffer
$finalScaledFiles = Get-ChildItem -Path $tempScaledDir -Filter "*.png" | Sort-Object Name
foreach ($file in $finalScaledFiles) {
    & magick "$($file.FullName)" -background none -gravity center -extent "$($frameWidth)x$($frameHeight)" "$($file.FullName)"
}

Write-Host "Building the grid via .NET Drawing..." -ForegroundColor Yellow

$rowCount = [math]::Ceiling($pngFiles.Count / $colCount)
$sheetWidth = $colCount * $frameWidth
$sheetHeight = $rowCount * $frameHeight
$outputPng = Join-Path $outputDir "$safeName.png"

$bitmap = New-Object System.Drawing.Bitmap($sheetWidth, $sheetHeight)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.Clear([System.Drawing.Color]::Transparent)

for ($i = 0; $i -lt $pngFiles.Count; $i++) {
    $col = $i % $colCount
    $row = [math]::Floor($i / $colCount)
    $x = $col * $frameWidth
    $y = $row * $frameHeight

    $scaledFilePath = Join-Path $tempScaledDir $pngFiles[$i].Name
    $img = [System.Drawing.Image]::FromFile($scaledFilePath)

    $graphics.DrawImage($img, $x, $y, $frameWidth, $frameHeight)
    $img.Dispose()
}

$bitmap.Save($outputPng, [System.Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose()
$bitmap.Dispose()

# 4. Build the JSON
$frames = @{}
for ($i = 0; $i -lt $pngFiles.Count; $i++) {
    $file = $pngFiles[$i]
    $col = $i % $colCount
    $row = [math]::Floor($i / $colCount)
    $x = $col * $frameWidth
    $y = $row * $frameHeight

    $frames[$file.Name] = @{
        frame = @{ x = $x; y = $y; w = $frameWidth; h = $frameHeight }
        rotated = $false
        trimmed = $false
        spriteSourceSize = @{ x = 0; y = 0; w = $frameWidth; h = $frameHeight }
        sourceSize = @{ w = $frameWidth; h = $frameHeight }
    }
}

$outJson = Join-Path $outputDir "$safeName.json"

$jsonObj = @{
    frames = $frames
    meta = @{
        app = "dotnet-drawing"
        version = "1.0"
        image = "$safeName.png"
        format = "RGBA8888"
        size = @{ w = $sheetWidth; h = $sheetHeight }
        scale = "1"
    }
}

$jsonObj | ConvertTo-Json -Depth 10 | Set-Content -Path $outJson

Remove-Item -Path $tempScaledDir -Recurse -Force

Write-Host "Successfully created $outputPng and $outputJson!" -ForegroundColor Green