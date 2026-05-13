package services

import (
	"context"
	"errors"
	"testing"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
)

func TestUserServiceGet(t *testing.T) {
	service := NewUserService()
	user, err := service.Get(context.Background(), " user1 ")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if user.NTAccount != "user1" || user.DisplayName != "Test User 測試員" {
		t.Fatalf("user = %+v", user)
	}
}

func TestUserServiceGetRejectsEmptyAccount(t *testing.T) {
	service := NewUserService()
	_, err := service.Get(context.Background(), " ")
	if !errors.Is(err, domainhr.ErrInvalidInput) {
		t.Fatalf("Get() error = %v, want ErrInvalidInput", err)
	}
}

func TestUserServiceBatchGetPreservesOrderAndDuplicates(t *testing.T) {
	service := NewUserService()
	users, err := service.BatchGet(context.Background(), []string{"user1", "user2", "user1"})
	if err != nil {
		t.Fatalf("BatchGet() error = %v", err)
	}
	if len(users) != 3 || users[0].NTAccount != "user1" || users[1].NTAccount != "user2" || users[2].NTAccount != "user1" {
		t.Fatalf("users = %+v", users)
	}
}

func TestUserServiceBatchGetRejectsEmptyList(t *testing.T) {
	service := NewUserService()
	_, err := service.BatchGet(context.Background(), nil)
	if !errors.Is(err, domainhr.ErrInvalidInput) {
		t.Fatalf("BatchGet() error = %v, want ErrInvalidInput", err)
	}
}
