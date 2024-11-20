package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("resource already exists")
	QueryTimeoutDuration = time.Second * 5
)

type Storage struct {
	Users interface {
		Create(context.Context, *User) error
		GetByUsername(context.Context, string) (*User, error)
		GetByID(context.Context, int64) (*User, error)
		// Delete(context.Context, int64) error
	}
	Images interface {
		Create(context.Context, *Image) error
		GetUserImages(context.Context, int64, PaginationParams) ([]Image, error)
		GetByID(context.Context, int64) (*Image, error)
		Update(context.Context, *Image) error
		// Delete(context.Context, int64) error
	}
}

func NewStorage(db *sql.DB) Storage {
	return Storage{
		Users:  &UserStore{db},
		Images: &ImageStore{db},
	}
}

func withTx(db *sql.DB, ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
