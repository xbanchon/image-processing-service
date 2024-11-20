package store

import (
	"context"
	"database/sql"
	"errors"
)

type Image struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	UserID    int64  `json:"user_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ImageStore struct {
	db *sql.DB
}

func (s ImageStore) Create(ctx context.Context, image *Image) error {
	query := `
			INSERT INTO images (url, filename, user_id)
			VALUES ($1, $2, $3)
			RETURNING id, created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(
		ctx,
		query,
		image.URL,
		image.Filename,
		image.UserID,
	).Scan(
		&image.ID,
		&image.CreatedAt,
		&image.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

func (s ImageStore) GetByID(ctx context.Context, id int64) (*Image, error) {
	query := `
			SELECT id, url, filename, user_id, created_at, updated_at
			FROM images
			WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	image := &Image{}

	err := s.db.QueryRowContext(
		ctx,
		query,
		id,
	).Scan(
		&image.ID,
		&image.URL,
		&image.Filename,
		&image.UserID,
		&image.CreatedAt,
		&image.UpdatedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrNotFound
		default:
			return nil, err
		}
	}

	return image, nil
}

func (s ImageStore) GetUserImages(ctx context.Context, userID int64, pp PaginationParams) ([]Image, error) {
	query := `
			SELECT id, url, filename, user_id, created_at, updated_at
			FROM images
			WHERE user_id = $1
			ORDER BY created_at
			LIMIT $2 OFFSET $3
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	offset := (pp.PageID - 1) * pp.Limit
	rows, err := s.db.QueryContext(ctx, query, userID, pp.Limit, offset)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var images []Image
	for rows.Next() {
		var i Image
		err := rows.Scan(
			&i.ID,
			&i.URL,
			&i.Filename,
			&i.UserID,
			&i.CreatedAt,
			&i.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		images = append(images, i)
	}

	return images, nil
}

func (s ImageStore) Update(ctx context.Context, image *Image) error {
	query := `
			UPDATE images
			SET url = $1, filename = $2, updated_at = $3
			WHERE id = $4
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	res, err := s.db.ExecContext(
		ctx,
		query,
		image.URL,
		image.Filename,
		image.UpdatedAt,
		image.ID,
	)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (s ImageStore) Delete(ctx context.Context, id int64) error {
	query := `
			DELETE FROM images
			WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
