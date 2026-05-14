package workspace

import "errors"

var (
	ErrInvalidInput = errors.New("invalid workspace input")
	ErrNotFound     = errors.New("workspace not found")
)
