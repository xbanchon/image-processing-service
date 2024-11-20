package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/xbanchon/image-processing-service/internal/store"
)

type Storage struct {
	Images interface {
		Get(context.Context, int64) (*store.Image, error)
		Set(context.Context, *store.Image) error
		Delete(context.Context, int64)
	}
}

func NewRedisStorage(rdb *redis.Client) Storage {
	return Storage{
		Images: &ImageStore{rdb: rdb},
	}
}
