package poc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
	clienthr "github.com/hao0731/workspace-permission-management/internal/shared/interactions/hr"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) clienthr.Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: http.DefaultClient,
	}
}

func (c *Client) Get(ctx context.Context, ntAccount string) (domainhr.User, error) {
	ntAccount = strings.TrimSpace(ntAccount)
	if ntAccount == "" {
		return domainhr.User{}, fmt.Errorf("nt account is required")
	}
	endpoint := c.baseURL + "/api/v1/users/" + url.PathEscape(ntAccount)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domainhr.User{}, fmt.Errorf("create hr get request: %w", err)
	}
	var response struct {
		User userDTO `json:"user"`
	}
	if err := c.do(req, &response); err != nil {
		return domainhr.User{}, err
	}
	return response.User.toDomain()
}

func (c *Client) BatchGet(ctx context.Context, ntAccounts []string) ([]domainhr.User, error) {
	normalized, err := normalizeAccounts(ntAccounts)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string][]string{"nt_accounts": normalized})
	if err != nil {
		return nil, fmt.Errorf("marshal hr batch request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/user-list", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create hr batch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	var response struct {
		Users []userDTO `json:"users"`
	}
	if err := c.do(req, &response); err != nil {
		return nil, err
	}
	users := make([]domainhr.User, 0, len(response.Users))
	for _, dto := range response.Users {
		user, err := dto.toDomain()
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (c *Client) do(req *http.Request, target any) (err error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send hr request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close hr response body: %w", closeErr)
		}
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("hr request failed with status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode hr response: %w", err)
	}
	return nil
}

type userDTO struct {
	NTAccount   string `json:"nt_account"`
	DisplayName string `json:"display_name"`
}

func (d userDTO) toDomain() (domainhr.User, error) {
	user := domainhr.User{NTAccount: d.NTAccount, DisplayName: d.DisplayName}.Normalize()
	if err := user.Validate(); err != nil {
		return domainhr.User{}, err
	}
	return user, nil
}

func normalizeAccounts(ntAccounts []string) ([]string, error) {
	if len(ntAccounts) == 0 {
		return nil, fmt.Errorf("nt accounts are required")
	}
	normalized := make([]string, 0, len(ntAccounts))
	for _, account := range ntAccounts {
		account = strings.TrimSpace(account)
		if account == "" {
			return nil, fmt.Errorf("nt account is required")
		}
		normalized = append(normalized, account)
	}
	return normalized, nil
}
