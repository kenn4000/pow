package skins

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"testing"
)

func TestShopSpriteSheetsContainCompleteSkinMetadata(t *testing.T) {
	sheets, err := ReadShopSheets(Files)
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) < 100 {
		t.Fatalf("found %d skin sprite sheets, want at least 100", len(sheets))
	}
	for _, sheet := range sheets {
		if sheet.Meta.SkinData.ID == "" || len(sheet.Frames) != len(requiredSpriteFrames) {
			t.Errorf("incomplete sprite sheet %s_%s", sheet.Family, sheet.Color)
		}
	}
	if _, err := Files.ReadFile("shop/tribal/red/small_idle.png"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("individual skin art should not be embedded alongside spritesheets")
	}
}

func TestCropSpriteFrameUsesMetadataRectangle(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 1))
	source.Set(0, 0, color.RGBA{R: 10, A: 255})
	source.Set(1, 0, color.RGBA{G: 20, A: 255})
	frame := SpriteFrame{
		Frame:      SpriteRect{X: 1, Y: 0, Width: 1, Height: 1},
		SourceSize: SpriteSize{Width: 1, Height: 1},
	}
	data, err := CropSpriteFrame(source, frame)
	if err != nil {
		t.Fatal(err)
	}
	cropped, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(cropped.At(0, 0)).(color.RGBA); got != (color.RGBA{G: 20, A: 255}) {
		t.Fatalf("cropped pixel = %#v, want green frame pixel", got)
	}
}
