package processor

import (
	"github.com/h2non/bimg"
)

var ImageTypes = map[string]bimg.ImageType{
	"jpeg": bimg.JPEG,
	"png":  bimg.PNG,
	"webp": bimg.WEBP,
	"tiff": bimg.TIFF,
}

type ImageProcessor struct {
	Transformer interface {
		Process() ([]byte, error)
		Resize(width, height int) ([]byte, error)
		Crop(width, height int) ([]byte, error)
		Convert(t string) ([]byte, error)
		Compress(value int) ([]byte, error)
		Rotate(a bimg.Angle) ([]byte, error)
		Mirror() ([]byte, error)
		Flip() ([]byte, error)
	}
}

func NewImageProcessor(buf []byte, options Transformer) *ImageProcessor {
	return &ImageProcessor{
		Transformer: &ImageTransformer{
			buf:     buf,
			options: options,
		},
	}
}
