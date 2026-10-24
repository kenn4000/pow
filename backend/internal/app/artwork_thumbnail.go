package app

import (
	"bytes"
	"image"
	"image/png"
	"sync"
)

type artworkThumbnail struct {
	once sync.Once
	data []byte
	err  error
}

var artworkThumbnails sync.Map

func thumbnailArtwork(name string, asset []byte, width int) ([]byte, error) {
	key := struct {
		name  string
		width int
	}{name, width}
	value, _ := artworkThumbnails.LoadOrStore(key, &artworkThumbnail{})
	thumbnail := value.(*artworkThumbnail)
	thumbnail.once.Do(func() {
		source, err := png.Decode(bytes.NewReader(asset))
		if err != nil {
			thumbnail.err = err
			return
		}
		bounds := source.Bounds()
		height := max(1, bounds.Dy()*width/bounds.Dx())
		resized := image.NewNRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				resized.Set(x, y, source.At(bounds.Min.X+x*bounds.Dx()/width, bounds.Min.Y+y*bounds.Dy()/height))
			}
		}
		var buffer bytes.Buffer
		thumbnail.err = png.Encode(&buffer, resized)
		thumbnail.data = buffer.Bytes()
	})
	return thumbnail.data, thumbnail.err
}
