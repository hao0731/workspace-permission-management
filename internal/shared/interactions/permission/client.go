package permission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
)

const (
	schemaWritePath        = "/api/v1/schema/write"
	relationshipsWritePath = "/api/v1/relationships/write"
)

type Client struct {
	baseURL         string
	apiKey          string
	apiKeyHeaderKey string
	httpClient      *http.Client
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func New(baseURL string, apiKey string, apiKeyHeaderKey string, opts ...Option) *Client {
	client := &Client{
		baseURL:         strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:          strings.TrimSpace(apiKey),
		apiKeyHeaderKey: strings.TrimSpace(apiKeyHeaderKey),
		httpClient:      http.DefaultClient,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

func (c *Client) RegisterResourceAttributes(ctx context.Context, systemID string, resourceAttributes []resource.ResourceAttribute) error {
	payload := newRegisterResourceAttributesRequest(systemID, resourceAttributes)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal permission API schema write request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+schemaWritePath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create permission API schema write request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(c.apiKeyHeaderKey, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send permission API request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	var errorResponse ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
		return fmt.Errorf("permission API request failed with status %d: decode permission API error response: %w", resp.StatusCode, err)
	}
	return &Error{StatusCode: resp.StatusCode, Response: errorResponse}
}

func (c *Client) WriteRelationships(ctx context.Context, parameter WriteRelationshipsParameter) (WriteRelationshipsResult, error) {
	payload := newWriteRelationshipsRequest(parameter)
	body, err := json.Marshal(payload)
	if err != nil {
		return WriteRelationshipsResult{}, fmt.Errorf("marshal permission API relationships write request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+relationshipsWritePath, bytes.NewReader(body))
	if err != nil {
		return WriteRelationshipsResult{}, fmt.Errorf("create permission API relationships write request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(c.apiKeyHeaderKey, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return WriteRelationshipsResult{}, fmt.Errorf("send permission API relationships write request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var errorResponse ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return WriteRelationshipsResult{}, fmt.Errorf("permission API relationships write request failed with status %d: decode permission API error response: %w", resp.StatusCode, err)
		}
		return WriteRelationshipsResult{}, &Error{StatusCode: resp.StatusCode, Response: errorResponse}
	}

	var response WriteRelationshipsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return WriteRelationshipsResult{}, fmt.Errorf("decode permission API relationships write response: %w", err)
	}
	return newWriteRelationshipsResult(response), nil
}
