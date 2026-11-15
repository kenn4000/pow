Add-Type -AssemblyName System.Drawing

$baseDir = "C:\Users\kenn\Documents\prisonersofwar\backend\internal\skins\hero_icons"
$outputDir = "C:\Users\kenn\Documents\prisonersofwar\output"

if (-Not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

$cardHeight = 256
$cardWidth = 256
$colCount = 4

# Find all folders in /temp that have PNGs
$skinFolders = Get-ChildItem -Path $baseDir -Directory -Recurse | Where-Object {
    (Get-ChildItem -Path $_.FullName -Filter "*.png").Count -gt 0
}

Write-Host "Found $($skinFolders.Count) folders to process..." -ForegroundColor Cyan

foreach ($folder in $skinFolders) {
    # Generate the safe name (e.g., "wwolf" or "plant")
    $relativePath = $folder.FullName.Substring((Resolve-Path $baseDir).Path.Length + 1)
    $safeName = $relativePath -replace '[\\/]', '_'

    Write-Host "Processing: $safeName" -ForegroundColor Yellow

    $tempScaledDir = Join-Path $folder.FullName "_scaled_temp"
    if (Test-Path $tempScaledDir) { Remove-Item -Path $tempScaledDir -Recurse -Force }
    New-Item -ItemType Directory -Path $tempScaledDir | Out-Null

    # Get images, ignoring test files
    $pngFiles = Get-ChildItem -Path $folder.FullName -Filter "*.png" | Where-Object { $_.Name -notlike "*test*" -and $_.Name -notlike "row_*" } | Sort-Object Name

    if ($pngFiles.Count -eq 0) {
        Remove-Item -Path $tempScaledDir -Recurse -Force
        continue
    }

    # 1. Force every image to be a perfect 366x512 card
    foreach ($file in $pngFiles) {
        $destFile = Join-Path $tempScaledDir $file.Name
        & magick "$($file.FullName)" -background none -resize "x$cardHeight" -gravity South -extent "$($cardWidth)x$($cardHeight)" "$destFile"
    }

    # 2. Build the grid using .NET Drawing
    $rowCount = [math]::Ceiling($pngFiles.Count / $colCount)
    $sheetWidth = $colCount * $cardWidth
    $sheetHeight = $rowCount * $cardHeight
    $outputPng = Join-Path $outputDir "$safeName.png"

    $bitmap = New-Object System.Drawing.Bitmap($sheetWidth, $sheetHeight)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.Clear([System.Drawing.Color]::Transparent)

    for ($i = 0; $i -lt $pngFiles.Count; $i++) {
        $file = $pngFiles[$i]
        $col = $i % $colCount
        $row = [math]::Floor($i / $colCount)
        $x = $col * $cardWidth
        $y = $row * $cardHeight

        $imgPath = Join-Path $tempScaledDir $file.Name
        $img = [System.Drawing.Image]::FromFile($imgPath)

        $graphics.DrawImage($img, $x, $y, $cardWidth, $cardHeight)
        $img.Dispose()
    }

    $bitmap.Save($outputPng, [System.Drawing.Imaging.ImageFormat]::Png)
    $graphics.Dispose()
    $bitmap.Dispose()

    # 3. Build JSON
    $frames = @{}
    for ($i = 0; $i -lt $pngFiles.Count; $i++) {
        $file = $pngFiles[$i]
        $col = $i % $colCount
        $row = [math]::Floor($i / $colCount)
        $x = $col * $cardWidth
        $y = $row * $cardHeight

        $frames[$file.Name] = @{
            frame = @{ x = $x; y = $y; w = $cardWidth; h = $cardHeight }
            rotated = $false
            trimmed = $false
            spriteSourceSize = @{ x = 0; y = 0; w = $cardWidth; h = $cardHeight }
            sourceSize = @{ w = $cardWidth; h = $cardHeight }
        }
    }

    $outJson = Join-Path $outputDir "$safeName.json"
    $skinJsonPath = Join-Path $folder.FullName "skin.json"

    # 4. Check for skin.json. If missing, create a template.
    $skinData = $null
    if (Test-Path $skinJsonPath) {
        $skinData = Get-Content -Path $skinJsonPath -Raw | ConvertFrom-Json
        Write-Host "  -> Found skin.json, merging data." -ForegroundColor Green
    } else {
        Write-Host "  -> No skin.json found. Generating template..." -ForegroundColor Magenta
        # Create a blank template for the user to fill out
        $template = @{
            id = $safeName
            displayName = "New Character"
            rarityRank = 0
            hexColor = "#FFFFFF"
            price = 0
            currency = "marks"
        }
        $template | ConvertTo-Json -Depth 5 | Set-Content -Path $skinJsonPath
    }

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

    # If skinData existed (or was just created), add it to the JSON
    if ($skinData) {
        $jsonObj.meta | Add-Member -MemberType NoteProperty -Name "skinData" -Value $skinData -Force
    }

    $jsonObj | ConvertTo-Json -Depth 10 | Set-Content -Path $outJson

    Remove-Item -Path $tempScaledDir -Recurse -Force
}

Write-Host "All skins processed successfully!" -ForegroundColor Green