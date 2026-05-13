package hr

import (
	"fmt"
	"strings"
)

func (u User) Normalize() User {
	u.NTAccount = strings.TrimSpace(u.NTAccount)
	u.DisplayName = strings.TrimSpace(u.DisplayName)
	return u
}

func (u User) Validate() error {
	normalized := u.Normalize()
	if normalized.NTAccount == "" {
		return invalidInput("nt account is required")
	}
	if normalized.DisplayName == "" {
		return invalidInput("display name is required")
	}
	return nil
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
