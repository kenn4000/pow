$folder = "C:\Users\kenn\Documents\prisonersofwar\backend\internal\skins\icons\chest"
$sourceImg = Join-Path $folder "prismatic_chest.png"

if (-Not (Test-Path $sourceImg)) {
    Write-Host "ERROR: Could not find 'wooden_chest.png' in $folder" -ForegroundColor Red
    exit
}

Write-Host "Generating shake frames..." -ForegroundColor Cyan

# The sequence of tilts
$shakeSequence = @(
    @{ angle = -3; name = "cart_shake_01.png" },
    @{ angle = 3;  name = "cart_shake_02.png" },
    @{ angle = -5; name = "cart_shake_03.png" },
    @{ angle = 5;  name = "cart_shake_04.png" },
    @{ angle = -2; name = "prismatic_shake_05.png" },
    @{ angle = 1;  name = "prismatic_shake_06.png" }
)

foreach ($frame in $shakeSequence) {
    $outPath = Join-Path $folder $frame.name
    $deg = $frame.angle

    Write-Host "Generating $($frame.name) ($deg degrees)..."

    # Rotate the image, remove the black background to make it transparent,
    # and automatically trim/extend the canvas so nothing gets cut off
    & magick "$sourceImg" -background none -rotate $deg -trim +repage "$outPath"
}

Write-Host "Done! Shake frames generated in $folder" -ForegroundColor Green