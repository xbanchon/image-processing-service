package store

import (
	"errors"
	"log"
	"net/http"
	"strconv"
)

var (
	ErrBadQuery = errors.New("malformed query")
)

type PaginationParams struct {
	PageID int `json:"page" validate:"gte=1"`
	Limit  int `json:"limit" validate:"gte=1"`
}

func (pp PaginationParams) Parse(r *http.Request) (PaginationParams, error) {
	q := r.URL.Query()
	log.Printf("Query: %+v", q)
	log.Printf("URL: %v", r.URL)
	if len(q) != 2 {
		return pp, ErrBadQuery
	}

	limit := q.Get("limit")
	if limit != "" {
		l, err := strconv.Atoi(limit)
		if err != nil {
			return pp, nil
		}
		pp.Limit = l
	}

	page := q.Get("page")
	if page != "" {
		p, err := strconv.Atoi(page)
		if err != nil {
			return pp, nil
		}

		pp.PageID = p
	}

	return pp, nil
}
