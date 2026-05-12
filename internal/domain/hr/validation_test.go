package hr

import (
	"errors"
	"testing"
)

func TestUserValidate(t *testing.T) {
	tests := []struct {
		name string
		user User
	}{
		{name: "missing nt account", user: User{DisplayName: "Test User 測試員"}},
		{name: "missing display name", user: User{NTAccount: "user1"}},
		{name: "blank nt account", user: User{NTAccount: " ", DisplayName: "Test User 測試員"}},
		{name: "blank display name", user: User{NTAccount: "user1", DisplayName: " "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.user.Validate(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Validate() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestUserNormalize(t *testing.T) {
	user := User{NTAccount: " user1 ", DisplayName: " Test User 測試員 "}
	normalized := user.Normalize()
	if normalized.NTAccount != "user1" || normalized.DisplayName != "Test User 測試員" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
}

func TestUserValidateAcceptsValidUser(t *testing.T) {
	user := User{NTAccount: "user1", DisplayName: "Test User 測試員"}
	if err := user.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
