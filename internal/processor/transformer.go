package processor

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"log"

	"github.com/disintegration/gift"
	"github.com/h2non/bimg"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"golang.org/x/image/tiff"
)

var (
	ErrInvalidParam = errors.New("invalid param value")
)

type ImageTransformer struct {
	buf     []byte
	options Transformer
}

type Transformer struct {
	Resize struct {
		Width  int
		Height int
	}
	Crop struct {
		Width  int
		Height int
	}
	Mirror  bool
	Flip    bool
	Rotate  int
	Quality int
	Format  string
	Filters struct {
		Grayscale    bool
		Sepia        bool
		Gamma        float32
		GaussianBlur float32
	}
}

func (it *ImageTransformer) Process() ([]byte, error) {
	imgQuality := it.options.Quality
	if imgQuality <= 0 {
		imgQuality = 75 //default value
	}

	bimgOpts := bimg.Options{
		Rotate:  bimg.Angle(it.options.Rotate),
		Flop:    it.options.Flip,
		Flip:    it.options.Mirror,
		Quality: imgQuality,
	}

	newBuf, err := bimg.NewImage(it.buf).Process(bimgOpts)
	if err != nil {
		return nil, err
	}

	// apply resize if neccessary
	rWidth := it.options.Resize.Width
	rHeight := it.options.Resize.Height
	if rWidth > 0 && rHeight > 0 {
		newBuf, err = bimg.NewImage(newBuf).Process(bimg.Options{
			Width:  rWidth,
			Height: rHeight,
		})
		if err != nil {
			return nil, err
		}
		log.Println("resizing done")
		// return nil, ErrInvalidParam
	}

	// apply crop if neccessary
	cWidth := it.options.Crop.Width
	cHeight := it.options.Crop.Height

	if cWidth > 0 && cHeight > 0 {
		newBuf, err = bimg.NewImage(newBuf).Process(bimg.Options{
			Width:   cWidth,
			Height:  cHeight,
			Gravity: bimg.GravityCentre,
		})
		if err != nil {
			return nil, err
		}
		log.Println("crop done")
		// return nil, ErrInvalidParam
	}

	//apply conversion if neccessary
	newType := it.options.Format
	if newType == "jpg" {
		newType = "jpeg"
	}
	if newType == "tif" {
		newType = "tiff"
	}

	bimgImgType, ok := ImageTypes[newType]
	if !ok && newType != "" {
		return nil, errors.New("unsorported image format")
	}

	if newType != "" {
		newBuf, err = bimg.NewImage(newBuf).Convert(bimgImgType)
		if err != nil {
			return nil, err
		}

		if bimg.DetermineImageTypeName(newBuf) != newType {
			return nil, errors.New("unknown conversion error")
		}
		log.Println("format conversion done")
	}

	//add filters if neccessary
	newBuf, err = addFilters(newBuf, it.options)
	if err != nil {
		return nil, err
	}
	// log.Println("filters application done")

	return newBuf, nil
}

func (it *ImageTransformer) Resize(width, height int) ([]byte, error) {
	newBuf, err := bimg.NewImage(it.buf).Resize(width, height)
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func (it *ImageTransformer) Crop(width, height int) ([]byte, error) {
	newBuf, err := bimg.NewImage(it.buf).Crop(width, height, bimg.GravityCentre) //Third parameter is Gravity
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func (it *ImageTransformer) Rotate(a bimg.Angle) ([]byte, error) {
	newBuf, err := bimg.NewImage(it.buf).Rotate(a)
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func (it *ImageTransformer) Flip() ([]byte, error) { //Reverse across horizontal axis
	newBuf, err := bimg.NewImage(it.buf).Flop()
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func (it *ImageTransformer) Mirror() ([]byte, error) { //Reverse across vertical axis
	newBuf, err := bimg.NewImage(it.buf).Flip()
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func (it *ImageTransformer) Compress(value int) ([]byte, error) {
	newBuf, err := bimg.NewImage(it.buf).Process(bimg.Options{
		Quality: value,
	})

	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func (it *ImageTransformer) Convert(t string) ([]byte, error) {
	imgType := bimg.DetermineImageTypeName(it.buf)

	if imgType == t {
		return it.buf, nil
	}

	newType, ok := ImageTypes[t]
	if !ok {
		return nil, errors.New("unsorported image format")
	}

	convertedImg, err := bimg.NewImage(it.buf).Convert(newType)
	if err != nil {
		return nil, err
	}

	if bimg.DetermineImageTypeName(convertedImg) != t {
		return nil, errors.New("unknown conversion error")
	}

	return convertedImg, nil
}

func addFilters(buf []byte, options Transformer) ([]byte, error) {
	imageType := bimg.DetermineImageTypeName(buf)
	filters := options.Filters

	g := gift.New()

	if filters.Grayscale {
		g.Add(gift.Grayscale())
	}

	if filters.Sepia {
		g.Add(gift.Sepia(50))
	}

	gamma := filters.Gamma
	if gamma > 0 {
		g.Add(gift.Gamma(gamma))
	}

	sigma := filters.GaussianBlur
	if sigma > 0 {
		g.Add(gift.GaussianBlur(sigma))
	}

	src, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	dst := image.NewNRGBA(g.Bounds(src.Bounds()))
	g.Draw(dst, src)

	newBuf, err := encodeImage(dst, imageType)
	if err != nil {
		return nil, err
	}

	return newBuf, nil
}

func encodeImage(dst image.Image, format string) ([]byte, error) {
	bufWriter := new(bytes.Buffer)

	switch format {
	case "jpeg", "jpg":
		if err := jpeg.Encode(bufWriter, dst, nil); err != nil {
			return nil, err
		}
	case "png":
		if err := png.Encode(bufWriter, dst); err != nil {
			return nil, err
		}
	case "webp":
		options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 75)
		if err != nil {
			return nil, err
		}
		if err := webp.Encode(bufWriter, dst, options); err != nil {
			return nil, err
		}
	case "tiff", "tif":
		if err := tiff.Encode(bufWriter, dst, nil); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsuported/bad image format")
	}

	return bufWriter.Bytes(), nil
}
