package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
)

type UserListRequest struct {
	NTAccounts []string `json:"nt_accounts"`
}

func DecodeUserListRequest(body io.Reader) (UserListRequest, error) {
	var raw struct {
		NTAccounts *[]string `json:"nt_accounts"`
	}
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&raw); err != nil {
		return UserListRequest{}, fmt.Errorf("decode user list request: %w", err)
	}
	if raw.NTAccounts == nil {
		return UserListRequest{}, invalidUserRequest("nt_accounts is required")
	}
	if len(*raw.NTAccounts) == 0 {
		return UserListRequest{}, invalidUserRequest("nt_accounts must not be empty")
	}
	return UserListRequest{NTAccounts: append([]string(nil), (*raw.NTAccounts)...)}, nil
}

func (request UserListRequest) ToDomain() ([]string, error) {
	if len(request.NTAccounts) == 0 {
		return nil, invalidUserRequest("nt_accounts must not be empty")
	}
	accounts := make([]string, 0, len(request.NTAccounts))
	for _, account := range request.NTAccounts {
		account = strings.TrimSpace(account)
		if account == "" {
			return nil, invalidUserRequest("nt_account is required")
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func invalidUserRequest(message string) error {
	return fmt.Errorf("%w: %s", domainhr.ErrInvalidInput, message)
}
