package supabase

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	sc "github.com/supabase-community/storage-go"
)

const urlDuration = 6 * 3600 // 6 hours

var ImageMIMETypes = map[string]string{
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
	"tiff": "image/tiff",
}

type ImageBucket struct {
	bucket_id string
	sc        *sc.Client
}

func (b ImageBucket) UploadImage(filename string, buf []byte) (string, string, error) {
	imgType := strings.Split(filename, ".")[1]
	if imgType == "jpg" {
		imgType = "jpeg"
	}
	if imgType == "tif" {
		imgType = "tiff"
	}

	newFilename := fmt.Sprintf("uploaded_%s", filename)
	var options sc.FileOptions

	if contentType, ok := ImageMIMETypes[imgType]; !ok {
		return "", "", errors.New("unsoported/bad image format")
	} else {
		options = sc.FileOptions{
			ContentType: &contentType,
		}
	}

	_, err := b.sc.UploadFile(b.bucket_id, newFilename, bytes.NewReader(buf), options)
	if err != nil {
		return "", "", err
	}

	//Gets signed URL valid for (duration int) seconds
	res, err := b.sc.CreateSignedUrl(b.bucket_id, newFilename, urlDuration)
	if err != nil {
		return "", "", err
	}

	return newFilename, res.SignedURL, nil
}

func (b ImageBucket) GetNewSignedImageURL(filename string, duration int) (string, error) {
	res, err := b.sc.CreateSignedUrl(b.bucket_id, filename, duration)
	if err != nil {
		return "", err
	}

	return res.SignedURL, nil
}

func (b ImageBucket) UpdateImage(filename string, buf []byte) error {
	imgType := strings.Split(filename, ".")[1]
	if imgType == "jpg" {
		imgType = "jpeg"
	}
	if imgType == "tif" {
		imgType = "tiff"
	}

	var options sc.FileOptions

	if contentType, ok := ImageMIMETypes[imgType]; !ok {
		return errors.New("unsoported/bad image format")
	} else {
		options = sc.FileOptions{
			ContentType: &contentType,
		}
	}

	_, err := b.sc.UpdateFile(b.bucket_id, filename, bytes.NewReader(buf), options)
	if err != nil {
		return err
	}
	return nil
}

func (b ImageBucket) StreamImage(filename string) ([]byte, error) {
	buf, err := b.sc.DownloadFile(b.bucket_id, filename)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
