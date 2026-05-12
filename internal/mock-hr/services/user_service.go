package services

import (
	"context"
	"fmt"
	"strings"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
)

const mockDisplayName = "Test User 測試員"

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) Get(_ context.Context, ntAccount string) (domainhr.User, error) {
	user := domainhr.User{NTAccount: strings.TrimSpace(ntAccount), DisplayName: mockDisplayName}.Normalize()
	if err := user.Validate(); err != nil {
		return domainhr.User{}, err
	}
	return user, nil
}

func (s *UserService) BatchGet(ctx context.Context, ntAccounts []string) ([]domainhr.User, error) {
	if len(ntAccounts) == 0 {
		return nil, fmt.Errorf("%w: nt_accounts is required", domainhr.ErrInvalidInput)
	}
	users := make([]domainhr.User, 0, len(ntAccounts))
	for _, account := range ntAccounts {
		user, err := s.Get(ctx, account)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
