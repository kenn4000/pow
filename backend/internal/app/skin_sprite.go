package app

import (
	"bytes"
	"container/list"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"path"
	"strings"
	"sync"

	"prisonersofwar/internal/skins"
)

type cachedSkinSpriteSheet struct {
	key     string
	mu      sync.Mutex
	loaded  bool
	sheet   skins.SpriteSheet
	image   image.Image
	frames  map[string][]byte
	loadErr error
}

const skinSpriteSheetCacheSize = 4

var skinSpriteSheets = struct {
	sync.Mutex
	entries map[string]*cachedSkinSpriteSheet
	order   *list.List
}{
	entries: make(map[string]*cachedSkinSpriteSheet),
	order:   list.New(),
}

func skinSpriteFrame(assetPath string) ([]byte, error) {
	parts := strings.Split(assetPath, "/")
	if len(parts) != 4 || parts[0] != "shop" || path.Ext(parts[3]) != ".png" {
		return nil, fs.ErrNotExist
	}
	key := parts[1] + "_" + parts[2]
	sheet := cachedSkinSheet(key)
	sheet.mu.Lock()
	defer sheet.mu.Unlock()
	if !sheet.loaded {
		sheet.loaded = true
		sheet.sheet, sheet.loadErr = skins.ReadShopSheet(webFiles, parts[1], parts[2])
		if sheet.loadErr == nil {
			imagePath := path.Join("skinshop", sheet.sheet.Meta.Image)
			var data []byte
			data, sheet.loadErr = webFiles.ReadFile(imagePath)
			if sheet.loadErr != nil {
				sheet.loadErr = fmt.Errorf("read skin sprite image %q: %w", imagePath, sheet.loadErr)
			} else {
				sheet.image, sheet.loadErr = png.Decode(bytes.NewReader(data))
				if sheet.loadErr != nil {
					sheet.loadErr = fmt.Errorf("decode skin sprite image %q: %w", imagePath, sheet.loadErr)
				}
			}
		}
		sheet.frames = make(map[string][]byte)
	}
	if sheet.loadErr != nil {
		return nil, sheet.loadErr
	}
	if data, ok := sheet.frames[parts[3]]; ok {
		return data, nil
	}
	sprite, ok := sheet.sheet.Frames[parts[3]]
	if !ok {
		return nil, fs.ErrNotExist
	}
	data, err := skins.CropSpriteFrame(sheet.image, sprite)
	if err != nil {
		return nil, err
	}
	sheet.frames[parts[3]] = data
	return data, nil
}

func cachedSkinSheet(key string) *cachedSkinSpriteSheet {
	skinSpriteSheets.Lock()
	defer skinSpriteSheets.Unlock()
	if sheet, ok := skinSpriteSheets.entries[key]; ok {
		for element := skinSpriteSheets.order.Front(); element != nil; element = element.Next() {
			if element.Value.(string) == key {
				skinSpriteSheets.order.MoveToFront(element)
				break
			}
		}
		return sheet
	}
	sheet := &cachedSkinSpriteSheet{key: key}
	skinSpriteSheets.entries[key] = sheet
	skinSpriteSheets.order.PushFront(key)
	if skinSpriteSheets.order.Len() > skinSpriteSheetCacheSize {
		oldest := skinSpriteSheets.order.Back()
		delete(skinSpriteSheets.entries, oldest.Value.(string))
		skinSpriteSheets.order.Remove(oldest)
	}
	return sheet
}
