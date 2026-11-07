package skins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

type SpriteSize struct {
	Width  int `json:"w"`
	Height int `json:"h"`
}

type SpriteRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"w"`
	Height int `json:"h"`
}

type SpriteFrame struct {
	Frame      SpriteRect `json:"frame"`
	Rotated    bool       `json:"rotated"`
	Trimmed    bool       `json:"trimmed"`
	SourceSize SpriteSize `json:"sourceSize"`
}

type SkinData struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
	HexColor    string `json:"hexColor"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency"`
	Rarity      string `json:"rarity"`
	RarityRank  int    `json:"rarityRank"`
}

type SpriteSheetMeta struct {
	Image    string     `json:"image"`
	Size     SpriteSize `json:"size"`
	SkinData SkinData   `json:"skinData"`
}

type SpriteSheet struct {
	Family string                 `json:"-"`
	Color  string                 `json:"-"`
	Frames map[string]SpriteFrame `json:"frames"`
	Meta   SpriteSheetMeta        `json:"meta"`
}

var spriteNamePart = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var requiredSpriteFrames = []string{
	"small_idle.png", "small_attack.png", "small_win.png", "small_lose.png",
	"med_idle.png", "med_attack.png", "med_win.png", "med_lose.png",
	"large_idle.png", "large_attack.png", "large_win.png", "large_lose.png",
}

func ReadShopSheets(assets fs.FS) ([]SpriteSheet, error) {
	entries, err := fs.ReadDir(assets, "skinshop")
	if err != nil {
		return nil, fmt.Errorf("read skin sprite sheets: %w", err)
	}
	sheets := make([]SpriteSheet, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), ".json")
		if strings.HasSuffix(stem, "__scaled") {
			continue
		}
		separator := strings.LastIndex(stem, "_")
		if separator <= 0 || separator == len(stem)-1 {
			return nil, fmt.Errorf("invalid skin sprite sheet name %q", entry.Name())
		}
		sheet, err := ReadShopSheet(assets, stem[:separator], stem[separator+1:])
		if err != nil {
			return nil, err
		}
		sheets = append(sheets, sheet)
	}
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no skin sprite sheets found")
	}
	return sheets, nil
}

func ReadShopSheet(assets fs.FS, family, color string) (SpriteSheet, error) {
	if !spriteNamePart.MatchString(family) || !spriteNamePart.MatchString(color) {
		return SpriteSheet{}, fmt.Errorf("invalid skin sprite sheet name %q_%q", family, color)
	}
	stem := family + "_" + color
	metadataPath := path.Join("skinshop", stem+".json")
	data, err := fs.ReadFile(assets, metadataPath)
	if err != nil {
		return SpriteSheet{}, fmt.Errorf("read skin sprite metadata %q: %w", metadataPath, err)
	}
	var sheet SpriteSheet
	if err := json.Unmarshal(data, &sheet); err != nil {
		return SpriteSheet{}, fmt.Errorf("decode skin sprite metadata %q: %w", metadataPath, err)
	}
	sheet.Family = family
	sheet.Color = color
	if sheet.Meta.Image != stem+".png" {
		return SpriteSheet{}, fmt.Errorf("skin sprite metadata %q references unexpected image %q", metadataPath, sheet.Meta.Image)
	}
	if sheet.Meta.Size.Width <= 0 || sheet.Meta.Size.Height <= 0 {
		return SpriteSheet{}, fmt.Errorf("skin sprite metadata %q has invalid dimensions", metadataPath)
	}
	if len(sheet.Frames) != len(requiredSpriteFrames) {
		return SpriteSheet{}, fmt.Errorf("skin sprite metadata %q has %d frames, want %d", metadataPath, len(sheet.Frames), len(requiredSpriteFrames))
	}
	for _, name := range requiredSpriteFrames {
		frame, ok := sheet.Frames[name]
		if !ok {
			return SpriteSheet{}, fmt.Errorf("skin sprite metadata %q is missing frame %q", metadataPath, name)
		}
		if frame.Rotated || frame.Trimmed || frame.Frame.Width <= 0 || frame.Frame.Height <= 0 ||
			frame.SourceSize.Width != frame.Frame.Width || frame.SourceSize.Height != frame.Frame.Height {
			return SpriteSheet{}, fmt.Errorf("skin sprite metadata %q has invalid frame %q", metadataPath, name)
		}
		if frame.Frame.X < 0 || frame.Frame.Y < 0 ||
			frame.Frame.X+frame.Frame.Width > sheet.Meta.Size.Width ||
			frame.Frame.Y+frame.Frame.Height > sheet.Meta.Size.Height {
			return SpriteSheet{}, fmt.Errorf("skin sprite metadata %q has out-of-bounds frame %q", metadataPath, name)
		}
	}
	imagePath := path.Join("skinshop", sheet.Meta.Image)
	imageFile, err := assets.Open(imagePath)
	if err != nil {
		return SpriteSheet{}, fmt.Errorf("open skin sprite image %q: %w", imagePath, err)
	}
	defer imageFile.Close()
	dimensions, err := png.DecodeConfig(imageFile)
	if err != nil {
		return SpriteSheet{}, fmt.Errorf("decode skin sprite image header %q: %w", imagePath, err)
	}
	if dimensions.Width != sheet.Meta.Size.Width || dimensions.Height != sheet.Meta.Size.Height {
		return SpriteSheet{}, fmt.Errorf("skin sprite image %q dimensions are %dx%d, metadata says %dx%d",
			imagePath, dimensions.Width, dimensions.Height, sheet.Meta.Size.Width, sheet.Meta.Size.Height)
	}
	return sheet, nil
}

func CropSpriteFrame(source image.Image, frame SpriteFrame) ([]byte, error) {
	if frame.Rotated || frame.Trimmed || frame.Frame.Width <= 0 || frame.Frame.Height <= 0 ||
		frame.SourceSize.Width != frame.Frame.Width || frame.SourceSize.Height != frame.Frame.Height {
		return nil, fmt.Errorf("invalid sprite frame")
	}
	sourceRect := image.Rect(frame.Frame.X, frame.Frame.Y, frame.Frame.X+frame.Frame.Width, frame.Frame.Y+frame.Frame.Height)
	if sourceRect.Min.X < source.Bounds().Min.X || sourceRect.Min.Y < source.Bounds().Min.Y ||
		sourceRect.Max.X > source.Bounds().Max.X || sourceRect.Max.Y > source.Bounds().Max.Y {
		return nil, fmt.Errorf("sprite frame is outside source image")
	}
	cropped := image.NewRGBA(image.Rect(0, 0, frame.Frame.Width, frame.Frame.Height))
	draw.Draw(cropped, cropped.Bounds(), source, sourceRect.Min, draw.Src)
	var output bytes.Buffer
	if err := png.Encode(&output, cropped); err != nil {
		return nil, fmt.Errorf("encode cropped sprite frame: %w", err)
	}
	return output.Bytes(), nil
}
