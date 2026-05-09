package group

import "errors"

var (
	ErrInvalidInput  = errors.New("invalid group input")
	ErrDuplicateName = errors.New("duplicate group name")
	ErrNotFound      = errors.New("group not found")
)
