package supabase

import (
	sc "github.com/supabase-community/storage-go"
)

type Storage struct {
	Images interface {
		UploadImage(filename string, buf []byte) (string, string, error)
		GetNewSignedImageURL(filename string, duration int) (string, error)
		UpdateImage(filename string, buf []byte) error
		StreamImage(filename string) ([]byte, error)
	}
}

func NewSupabaseStorage(sc *sc.Client) Storage {
	return Storage{
		Images: &ImageBucket{"transformed-images", sc},
	}
}
