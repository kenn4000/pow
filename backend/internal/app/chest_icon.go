package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"path"
	"strings"
	"sync"

	"prisonersofwar/internal/skins"
)

type chestIconDefinition struct {
	sheet string
	frame string
}

type chestIconAtlas struct {
	Frames map[string]skins.SpriteFrame `json:"frames"`
	Meta   struct {
		Image string           `json:"image"`
		Size  skins.SpriteSize `json:"size"`
	} `json:"meta"`
}

type chestIconCacheEntry struct {
	once sync.Once
	data []byte
	err  error
}

var chestIconDefinitions = map[string]chestIconDefinition{
	"icons/wooden_chest.png":    {sheet: "wooden_chest_anim", frame: "wooden_chest.png"},
	"icons/silver_chest.png":    {sheet: "silver_chest_anim", frame: "silver_chest.png"},
	"icons/golden_chest.png":    {sheet: "golden_chest_anim", frame: "golden_chest.png"},
	"icons/prismatic_chest.png": {sheet: "prismatic_chest_anim", frame: "prismatic_chest.png"},
}

var chestIconCache sync.Map

func chestIconSpriteFrame(assetPath string) ([]byte, bool, error) {
	definition, ok := chestIconDefinitions[assetPath]
	if !ok {
		return nil, false, nil
	}
	value, _ := chestIconCache.LoadOrStore(assetPath, &chestIconCacheEntry{})
	entry := value.(*chestIconCacheEntry)
	entry.once.Do(func() {
		metadataPath := path.Join("icons", "animations", definition.sheet+".json")
		metadata, err := skins.IconFiles.ReadFile(metadataPath)
		if err != nil {
			entry.err = fmt.Errorf("read chest sprite metadata %q: %w", metadataPath, err)
			return
		}
		var atlas chestIconAtlas
		if err := json.Unmarshal(metadata, &atlas); err != nil {
			entry.err = fmt.Errorf("decode chest sprite metadata %q: %w", metadataPath, err)
			return
		}
		if atlas.Meta.Image != definition.sheet+".png" ||
			atlas.Meta.Size.Width <= 0 || atlas.Meta.Size.Height <= 0 {
			entry.err = fmt.Errorf("invalid chest sprite metadata %q", metadataPath)
			return
		}
		frame, ok := atlas.Frames[definition.frame]
		if !ok {
			entry.err = fmt.Errorf("chest sprite metadata %q is missing frame %q", metadataPath, definition.frame)
			return
		}
		imagePath := path.Join("icons", "animations", atlas.Meta.Image)
		imageData, err := skins.IconFiles.ReadFile(imagePath)
		if err != nil {
			entry.err = fmt.Errorf("read chest sprite image %q: %w", imagePath, err)
			return
		}
		image, err := png.Decode(bytes.NewReader(imageData))
		if err != nil {
			entry.err = fmt.Errorf("decode chest sprite image %q: %w", imagePath, err)
			return
		}
		if image.Bounds().Dx() != atlas.Meta.Size.Width || image.Bounds().Dy() != atlas.Meta.Size.Height {
			entry.err = fmt.Errorf("chest sprite image %q dimensions differ from metadata", imagePath)
			return
		}
		entry.data, entry.err = skins.CropSpriteFrame(image, frame)
		if entry.err != nil {
			entry.err = fmt.Errorf("crop chest sprite frame %q: %w", definition.frame, entry.err)
		}
	})
	return entry.data, true, entry.err
}

func isChestIconAsset(assetPath string) bool {
	return strings.HasPrefix(assetPath, "icons/") && chestIconDefinitions[assetPath].sheet != ""
}
