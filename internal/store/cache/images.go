package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xbanchon/image-processing-service/internal/store"
)

type ImageStore struct {
	rdb *redis.Client
}

const ImageExpTime = 15 * time.Minute

func (s *ImageStore) Get(ctx context.Context, imageID int64) (*store.Image, error) {
	cacheKey := fmt.Sprintf("image-%d", imageID)

	data, err := s.rdb.Get(ctx, cacheKey).Result() //Check this logic
	switch {
	case err == redis.Nil, data == "":
	case err != nil:
		return nil, err
	}

	if data != "" {
		var image store.Image
		if err := json.Unmarshal([]byte(data), &image); err != nil {
			return nil, err
		}
		return &image, nil
	}

	return nil, nil
}

func (s *ImageStore) Set(ctx context.Context, image *store.Image) error {
	cacheKey := fmt.Sprintf("image-%d", image.ID)

	json, err := json.Marshal(image)
	if err != nil {
		return nil
	}

	return s.rdb.SetEx(ctx, cacheKey, json, ImageExpTime).Err()

}

func (s *ImageStore) Delete(ctx context.Context, imageID int64) {
	cacheKey := fmt.Sprintf("image-%d", imageID)

	s.rdb.Del(ctx, cacheKey)
}
