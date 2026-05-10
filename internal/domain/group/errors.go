package group

import "errors"

var (
	ErrInvalidInput    = errors.New("invalid group input")
	ErrDuplicateName   = errors.New("duplicate group name")
	ErrDuplicateMember = errors.New("duplicate group individual member")
	ErrNotFound        = errors.New("group not found")
)
