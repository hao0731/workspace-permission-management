package pagination

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
)

const (
	DefaultLimit = 20
	MaxLimit     = 50
)

type PaginationHelper struct {
	defaultLimit int
	maxLimit     int
}

type Option func(*PaginationHelper)

func WithDefaultLimit(limit int) Option {
	return func(h *PaginationHelper) {
		h.defaultLimit = limit
	}
}

func WithMaxLimit(limit int) Option {
	return func(h *PaginationHelper) {
		h.maxLimit = limit
	}
}

func New(opts ...Option) *PaginationHelper {
	h := &PaginationHelper{defaultLimit: DefaultLimit, maxLimit: MaxLimit}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *PaginationHelper) ParseLimit(c *echo.Context) (int, error) {
	raw := c.QueryParam("limit")
	if strings.TrimSpace(raw) == "" {
		return h.defaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if limit < 1 {
		return 0, fmt.Errorf("limit must be greater than zero")
	}
	if limit > h.maxLimit {
		return 0, fmt.Errorf("limit must be less than or equal to %d", h.maxLimit)
	}
	return limit, nil
}

func (h *PaginationHelper) ParseToken(c *echo.Context) (string, error) {
	return c.QueryParam("next_token"), nil
}
