# Permission API Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the temporary permission in-memory runtime wiring with an HTTP permission API client and add a mock permission API service for local integration.

**Architecture:** Keep `internal/shared/interactions/permission.Client` as the service-facing interface. Add `internal/shared/interactions/permission/api` as the concrete HTTP implementation, wire it from `cmd/function-service/main.go` through validated config, and add `mock-permission-api` as a thin Echo service that logs schema-write payloads and returns success.

**Tech Stack:** Go 1.25, standard `net/http`, `encoding/json`, Echo v5, `log/slog`, `viper`, existing `internal/domain/resource`, existing `internal/shared/health`.

---

## Source Designs And Policies

- Source design: [../../designs/permission-api-client-design.md](../../designs/permission-api-client-design.md)
- Related design: [../../designs/permission-client-design.md](../../designs/permission-client-design.md)
- Related design: [../../designs/function-service-system-resource-api-design.md](../../designs/function-service-system-resource-api-design.md)
- Related design: [../../designs/inmemory-permission-client-design.md](../../designs/inmemory-permission-client-design.md)
- Policy: [../../policies/backend-architecture-principle.md](../../policies/backend-architecture-principle.md)
- Policy: [../../policies/design-and-plan-docs-policy.md](../../policies/design-and-plan-docs-policy.md)

Policy summary:

- This is backend implementation plus design-plan documentation work.
- Remote HTTP transport details must stay inside `internal/shared/interactions/permission/api`.
- `function-service` services continue to depend on the shared permission interface, not HTTP DTOs or `net/http`.
- `mock-permission-api` handlers stay thin and only decode, log, and render responses.
- Configuration must come from environment variables and required values must be validated explicitly.
- REST API contract changes need an executable `.http` example under `examples/api/`.
- This plan starts under `docs/plans/active/`, links its source designs, and must be moved to `docs/plans/completed/` after implementation.

## File Structure And Responsibilities

- Create: `internal/shared/interactions/permission/api/request.go`
  - Remote schema-write request DTOs and fixed relation constants.
- Create: `internal/shared/interactions/permission/api/errors.go`
  - Permission API error response DTO and non-2xx client error type.
- Create: `internal/shared/interactions/permission/api/client.go`
  - Concrete HTTP client implementing `permission.Client`.
- Create: `internal/shared/interactions/permission/api/client_test.go`
  - API client request, header, payload, success, failure, and no-mutation tests.
- Modify: `internal/function-service/config/config.go`
  - Permission API config group, URL/header validation, and environment loading.
- Modify: `internal/function-service/config/config_test.go`
  - Required permission API config tests.
- Modify: `cmd/function-service/main.go`
  - Replace in-memory runtime wiring with the API client.
- Modify: `cmd/function-service/main_test.go`
  - Verify the composition helper returns the API client.
- Create: `internal/mock-permission-api/config/config.go`
  - Mock service environment config.
- Create: `internal/mock-permission-api/config/config_test.go`
  - Mock service config validation tests.
- Create: `internal/mock-permission-api/handlers/schema_handler.go`
  - `POST /api/v1/schema/write` route and permission API error response rendering.
- Create: `internal/mock-permission-api/handlers/schema_handler_test.go`
  - Success and malformed JSON handler tests.
- Create: `cmd/mock-permission-api/main.go`
  - Mock service composition root.
- Create: `cmd/mock-permission-api/main_test.go`
  - Health route smoke test.
- Modify: `.env`
  - Local permission API and mock permission API variables.
- Modify: `.env.example`
  - Document required permission API and mock permission API variables.
- Modify: `docker-compose.yml`
  - Add only the `mock-permission-api` service.
- Create: `examples/api/mock_permission_api.http`
  - Manual REST Client examples for schema-write success and malformed JSON.
- Move after implementation: `docs/plans/active/2026-05-19-permission-api-client.md`
  - Move to `docs/plans/completed/2026-05-19-permission-api-client.md`.

---

### Task 1: Permission API Client

**Files:**

- Create: `internal/shared/interactions/permission/api/client_test.go`
- Create: `internal/shared/interactions/permission/api/request.go`
- Create: `internal/shared/interactions/permission/api/errors.go`
- Create: `internal/shared/interactions/permission/api/client.go`

- [ ] **Step 1: Write failing API client tests**

Create `internal/shared/interactions/permission/api/client_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

var _ clientpermission.Client = (*Client)(nil)

func TestClientRegisterResourceAttributesSendsSchemaWriteRequest(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotContentType string
	var gotAPIKey string
	var gotRequest RegisterResourceAttributesRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotAPIKey = r.Header.Get("X-API-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	attrs := []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
		resource.ResourceAttribute("can_view_public_repo"),
	}
	client := New(" "+server.URL+"/ ", "secret-key", "X-API-Key")

	if err := client.RegisterResourceAttributes(context.Background(), "todo", attrs); err != nil {
		t.Fatalf("RegisterResourceAttributes error = %v, want nil", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/schema/write" {
		t.Fatalf("path = %q, want /api/v1/schema/write", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotAPIKey != "secret-key" {
		t.Fatalf("X-API-Key = %q, want secret-key", gotAPIKey)
	}
	if gotRequest.Definition != "todo" {
		t.Fatalf("definition = %q, want todo", gotRequest.Definition)
	}
	if len(gotRequest.Relations) != 2 {
		t.Fatalf("relations len = %d, want 2", len(gotRequest.Relations))
	}
	if gotRequest.Relations[0].ResourceAttribute != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("first resAttr = %q", gotRequest.Relations[0].ResourceAttribute)
	}
	if gotRequest.Relations[0].Condition != "enable_dynamic_context" {
		t.Fatalf("condition = %q, want enable_dynamic_context", gotRequest.Relations[0].Condition)
	}
	if gotRequest.Relations[0].IsPublic {
		t.Fatal("isPublic = true, want false")
	}
	if attrs[0] != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("attrs mutated to %#v", attrs)
	}
}

func TestClientRegisterResourceAttributesReturnsAPIErrorForNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Code:    400,
			Error:   "validation_failed",
			Message: "Invalid schema write payload",
		})
	}))
	defer server.Close()

	client := New(server.URL, "secret-key", "X-API-Key")
	err := client.RegisterResourceAttributes(context.Background(), "todo", []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})
	if err == nil {
		t.Fatal("RegisterResourceAttributes error = nil, want error")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Response.Error != "validation_failed" || apiErr.Response.Message != "Invalid schema write payload" {
		t.Fatalf("response = %+v", apiErr.Response)
	}
}

func TestClientRegisterResourceAttributesReturnsDecodeErrorForMalformedErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := New(server.URL, "secret-key", "X-API-Key")
	err := client.RegisterResourceAttributes(context.Background(), "todo", []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})
	if err == nil {
		t.Fatal("RegisterResourceAttributes error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode permission API error response") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func TestClientRegisterResourceAttributesReturnsRequestFailure(t *testing.T) {
	client := New("http://permission.local", "secret-key", "X-API-Key", WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}),
	}))

	err := client.RegisterResourceAttributes(context.Background(), "todo", []resource.ResourceAttribute{
		resource.ResourceAttribute("can_edit_private_repo"),
	})
	if err == nil {
		t.Fatal("RegisterResourceAttributes error = nil, want error")
	}
	if !strings.Contains(err.Error(), "send permission API request") {
		t.Fatalf("error = %q, want send context", err.Error())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
```

- [ ] **Step 2: Run targeted tests and confirm failure**

Run:

```bash
go test ./internal/shared/interactions/permission/api -run 'TestClient' -v
```

Expected: FAIL because `internal/shared/interactions/permission/api` does not exist and `Client`, `New`, `Error`, and request DTOs are undefined.

- [ ] **Step 3: Add schema-write request DTOs**

Create `internal/shared/interactions/permission/api/request.go`:

```go
package api

import "github.com/hao0731/workspace-permission-management/internal/domain/resource"

const dynamicContextCondition = "enable_dynamic_context"

type RegisterResourceAttributesRelationRequest struct {
	ResourceAttribute resource.ResourceAttribute `json:"resAttr"`
	Condition         string                     `json:"condition"`
	IsPublic          bool                       `json:"isPublic"`
}

type RegisterResourceAttributesRequest struct {
	Definition string                                      `json:"definition"`
	Relations  []RegisterResourceAttributesRelationRequest `json:"relations"`
}

func newRegisterResourceAttributesRequest(systemID string, resourceAttributes []resource.ResourceAttribute) RegisterResourceAttributesRequest {
	relations := make([]RegisterResourceAttributesRelationRequest, 0, len(resourceAttributes))
	for _, attribute := range resourceAttributes {
		relations = append(relations, RegisterResourceAttributesRelationRequest{
			ResourceAttribute: attribute,
			Condition:         dynamicContextCondition,
			IsPublic:          false,
		})
	}
	return RegisterResourceAttributesRequest{
		Definition: systemID,
		Relations:  relations,
	}
}
```

- [ ] **Step 4: Add permission API error types**

Create `internal/shared/interactions/permission/api/errors.go`:

```go
package api

import "fmt"

type ErrorResponse struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type Error struct {
	StatusCode int
	Response   ErrorResponse
}

func (e *Error) Error() string {
	if e.Response.Message != "" {
		return fmt.Sprintf("permission API request failed with status %d: %s", e.StatusCode, e.Response.Message)
	}
	return fmt.Sprintf("permission API request failed with status %d", e.StatusCode)
}
```

- [ ] **Step 5: Add the HTTP client implementation**

Create `internal/shared/interactions/permission/api/client.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

const schemaWritePath = "/api/v1/schema/write"

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

func New(baseURL string, apiKey string, apiKeyHeaderKey string, opts ...Option) clientpermission.Client {
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
```

- [ ] **Step 6: Run API client tests**

Run:

```bash
go test ./internal/shared/interactions/permission/api -v
```

Expected: PASS for all API client tests.

- [ ] **Step 7: Run shared permission package tests**

Run:

```bash
go test ./internal/shared/interactions/permission/... -v
```

Expected: PASS for `permission`, `permission/inmemory`, and `permission/api`.

- [ ] **Step 8: Commit API client**

Run:

```bash
git add internal/shared/interactions/permission/api
git commit -m "feat: add permission API client"
```

Expected: commit succeeds and includes only the API client package files.

---

### Task 2: Function Service Config And Runtime Wiring

**Files:**

- Modify: `internal/function-service/config/config.go`
- Modify: `internal/function-service/config/config_test.go`
- Modify: `cmd/function-service/main.go`
- Modify: `cmd/function-service/main_test.go`

- [ ] **Step 1: Add failing config tests**

In `internal/function-service/config/config_test.go`, add this helper near the top of the file after imports:

```go
func setRequiredFunctionServiceEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FUNCTION_SERVICE_HTTP_ADDR", ":8080")
	t.Setenv("FUNCTION_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("FUNCTION_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("FUNCTION_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_STREAM", "FUNCTION_RESOURCES")
	t.Setenv("FUNCTION_SERVICE_JETSTREAM_DURABLE", "function-service-resource-upserter")
	t.Setenv("FUNCTION_SERVICE_PERMISSION_API_BASE_URL", "http://localhost:8086")
	t.Setenv("FUNCTION_SERVICE_PERMISSION_API_KEY", "dev-permission-api-key")
	t.Setenv("FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER", "X-API-Key")
}
```

Then update the existing tests so they call `setRequiredFunctionServiceEnv(t)` before setting test-specific overrides. In `TestLoadReadsRequiredEnvironment`, also set the three permission API env vars explicitly:

```go
t.Setenv("FUNCTION_SERVICE_PERMISSION_API_BASE_URL", "https://permission.example.com")
t.Setenv("FUNCTION_SERVICE_PERMISSION_API_KEY", "prod-key")
t.Setenv("FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER", "X-Permission-Key")
```

Add assertions to `TestLoadReadsRequiredEnvironment`:

```go
if cfg.PermissionAPI.BaseURL != "https://permission.example.com" {
	t.Fatalf("PermissionAPI.BaseURL = %q, want https://permission.example.com", cfg.PermissionAPI.BaseURL)
}
if cfg.PermissionAPI.APIKey != "prod-key" {
	t.Fatalf("PermissionAPI.APIKey = %q, want prod-key", cfg.PermissionAPI.APIKey)
}
if cfg.PermissionAPI.APIKeyHeader != "X-Permission-Key" {
	t.Fatalf("PermissionAPI.APIKeyHeader = %q, want X-Permission-Key", cfg.PermissionAPI.APIKeyHeader)
}
```

Add these new tests at the end of the file:

```go
func TestLoadRejectsInvalidPermissionAPIBaseURL(t *testing.T) {
	setRequiredFunctionServiceEnv(t)
	t.Setenv("FUNCTION_SERVICE_PERMISSION_API_BASE_URL", "localhost:8086")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsBlankPermissionAPIKey(t *testing.T) {
	setRequiredFunctionServiceEnv(t)
	t.Setenv("FUNCTION_SERVICE_PERMISSION_API_KEY", " ")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsInvalidPermissionAPIKeyHeader(t *testing.T) {
	setRequiredFunctionServiceEnv(t)
	t.Setenv("FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER", "Bad Header")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}
```

- [ ] **Step 2: Run config tests and confirm failure**

Run:

```bash
go test ./internal/function-service/config -run 'TestLoad' -v
```

Expected: FAIL because `Config.PermissionAPI` and permission API validation do not exist yet.

- [ ] **Step 3: Add permission API config loading and validation**

In `internal/function-service/config/config.go`, add `net/url` to the standard library imports:

```go
import (
	"fmt"
	"net/url"
	"strings"
	"time"
)
```

Add `PermissionAPI PermissionAPIConfig` to `Config`:

```go
type Config struct {
	Environment            environment.Environment
	HTTPAddr               string
	MongoDB                MongoDBConfig
	NATS                   NATSConfig
	JetStream              JetStreamConfig
	SystemResourceLimits   SystemResourceLimitsConfig
	PermissionAPI          PermissionAPIConfig
	ResourceDeletedSubject string
	ShutdownTimeout        time.Duration
}
```

Add the new config type after `SystemResourceLimitsConfig`:

```go
type PermissionAPIConfig struct {
	BaseURL      string
	APIKey       string
	APIKeyHeader string
}
```

In `Load`, populate `PermissionAPI`:

```go
PermissionAPI: PermissionAPIConfig{
	BaseURL:      v.GetString("FUNCTION_SERVICE_PERMISSION_API_BASE_URL"),
	APIKey:       v.GetString("FUNCTION_SERVICE_PERMISSION_API_KEY"),
	APIKeyHeader: v.GetString("FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER"),
},
```

In `Validate`, add the three required values to the `required` map:

```go
"FUNCTION_SERVICE_PERMISSION_API_BASE_URL":   c.PermissionAPI.BaseURL,
"FUNCTION_SERVICE_PERMISSION_API_KEY":        c.PermissionAPI.APIKey,
"FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER": c.PermissionAPI.APIKeyHeader,
```

After the existing limit checks and before duration checks, add:

```go
if err := validatePermissionAPIBaseURL(c.PermissionAPI.BaseURL); err != nil {
	return err
}
if !isHTTPHeaderName(strings.TrimSpace(c.PermissionAPI.APIKeyHeader)) {
	return fmt.Errorf("FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER must be a valid HTTP header name")
}
```

Add helper functions at the bottom of the file:

```go
func validatePermissionAPIBaseURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("FUNCTION_SERVICE_PERMISSION_API_BASE_URL must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("FUNCTION_SERVICE_PERMISSION_API_BASE_URL must be an absolute http or https URL")
	}
	return nil
}

func isHTTPHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if strings.ContainsRune("!#$%&'*+-.^_`|~", r) {
			continue
		}
		return false
	}
	return true
}
```

- [ ] **Step 4: Run config tests**

Run:

```bash
go test ./internal/function-service/config -v
```

Expected: PASS.

- [ ] **Step 5: Write failing composition test for API client wiring**

In `cmd/function-service/main_test.go`, add imports:

```go
functionconfig "github.com/hao0731/workspace-permission-management/internal/function-service/config"
permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
```

Add this test:

```go
func TestNewPermissionClientReturnsAPIClient(t *testing.T) {
	client := newPermissionClient(functionconfig.PermissionAPIConfig{
		BaseURL:      "http://localhost:8086",
		APIKey:       "dev-permission-api-key",
		APIKeyHeader: "X-API-Key",
	})
	if _, ok := client.(*permissionapi.Client); !ok {
		t.Fatalf("permission client type = %T, want *api.Client", client)
	}
}
```

- [ ] **Step 6: Run function-service command tests and confirm failure**

Run:

```bash
go test ./cmd/function-service -run 'TestNewPermissionClientReturnsAPIClient' -v
```

Expected: FAIL because `newPermissionClient` is undefined.

- [ ] **Step 7: Replace in-memory runtime wiring with API client**

In `cmd/function-service/main.go`, replace the in-memory import:

```go
permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
```

Remove:

```go
permissioninmemory "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/inmemory"
```

Add this helper below `processIndicator`:

```go
func newPermissionClient(cfg config.PermissionAPIConfig) clientpermission.Client {
	return permissionapi.New(cfg.BaseURL, cfg.APIKey, cfg.APIKeyHeader)
}
```

Before `NewSystemResourceService`, create the client:

```go
permissionClient := newPermissionClient(cfg.PermissionAPI)
```

Then pass `permissionClient` instead of the in-memory constructor:

```go
systemResourceService := services.NewSystemResourceService(systemResourceRepository, resource.ResourceDefinitionLimits{
	Types:   cfg.SystemResourceLimits.Type,
	Actions: cfg.SystemResourceLimits.Action,
	Tags:    cfg.SystemResourceLimits.Tag,
}, permissionClient)
```

- [ ] **Step 8: Run function-service tests**

Run:

```bash
go test ./internal/function-service/... ./cmd/function-service -v
```

Expected: PASS.

- [ ] **Step 9: Commit function-service config and wiring**

Run:

```bash
git add internal/function-service/config cmd/function-service
git commit -m "feat: wire function service permission API client"
```

Expected: commit succeeds and includes only function-service config and command wiring files.

---

### Task 3: Mock Permission API Service

**Files:**

- Create: `internal/mock-permission-api/config/config_test.go`
- Create: `internal/mock-permission-api/config/config.go`
- Create: `internal/mock-permission-api/handlers/schema_handler_test.go`
- Create: `internal/mock-permission-api/handlers/schema_handler.go`
- Create: `cmd/mock-permission-api/main_test.go`
- Create: `cmd/mock-permission-api/main.go`

- [ ] **Step 1: Write failing mock service config tests**

Create `internal/mock-permission-api/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestConfigValidateRequiresHTTPAddr(t *testing.T) {
	cfg := Config{Environment: environment.Development, ShutdownTimeout: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateRejectsInvalidEnvironment(t *testing.T) {
	cfg := Config{Environment: environment.Environment("invalid"), HTTPAddr: ":8086", ShutdownTimeout: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateRequiresPositiveShutdownTimeout(t *testing.T) {
	cfg := Config{Environment: environment.Development, HTTPAddr: ":8086"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	cfg := Config{Environment: environment.Development, HTTPAddr: ":8086", ShutdownTimeout: time.Second}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
```

- [ ] **Step 2: Run mock service config tests and confirm failure**

Run:

```bash
go test ./internal/mock-permission-api/config -v
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Add mock service config**

Create `internal/mock-permission-api/config/config.go`:

```go
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Config struct {
	Environment     environment.Environment
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("MOCK_PERMISSION_API_ENV", string(environment.Development))
	v.SetDefault("MOCK_PERMISSION_API_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment:     environment.Environment(v.GetString("MOCK_PERMISSION_API_ENV")),
		HTTPAddr:        v.GetString("MOCK_PERMISSION_API_HTTP_ADDR"),
		ShutdownTimeout: v.GetDuration("MOCK_PERMISSION_API_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: MOCK_PERMISSION_API_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("MOCK_PERMISSION_API_HTTP_ADDR is required")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("MOCK_PERMISSION_API_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}
```

- [ ] **Step 4: Run mock service config tests**

Run:

```bash
go test ./internal/mock-permission-api/config -v
```

Expected: PASS.

- [ ] **Step 5: Write failing mock schema handler tests**

Create `internal/mock-permission-api/handlers/schema_handler_test.go`:

```go
package handlers

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestWriteSchemaLogsPayloadAndReturnsOK(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

	e := echo.New()
	RegisterRoutes(e, NewSchemaHandler(logger))

	body := `{"definition":"todo","relations":[{"resAttr":"can_edit_private_repo","condition":"enable_dynamic_context","isPublic":false}]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/schema/write", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	output := logBuffer.String()
	if !strings.Contains(output, "mock permission schema write received") {
		t.Fatalf("log output = %q, want schema write message", output)
	}
	if !strings.Contains(output, "relation_count=1") {
		t.Fatalf("log output = %q, want relation_count", output)
	}
}

func TestWriteSchemaRejectsMalformedJSON(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, NewSchemaHandler(slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/schema/write", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"validation_failed"`) {
		t.Fatalf("body = %s, want validation error", rec.Body.String())
	}
}
```

- [ ] **Step 6: Run mock handler tests and confirm failure**

Run:

```bash
go test ./internal/mock-permission-api/handlers -v
```

Expected: FAIL because the handler package does not exist.

- [ ] **Step 7: Add mock schema handler**

Create `internal/mock-permission-api/handlers/schema_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	permissionapi "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api"
)

type SchemaHandler struct {
	logger *slog.Logger
}

func NewSchemaHandler(logger *slog.Logger) *SchemaHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SchemaHandler{logger: logger}
}

func RegisterRoutes(e *echo.Echo, handler *SchemaHandler) {
	e.POST("/api/v1/schema/write", handler.WriteSchema)
}

func (h *SchemaHandler) WriteSchema(c *echo.Context) error {
	var request permissionapi.RegisterResourceAttributesRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&request); err != nil {
		return c.JSON(http.StatusBadRequest, permissionapi.ErrorResponse{
			Code:    http.StatusBadRequest,
			Error:   "validation_failed",
			Message: "Invalid schema write payload",
		})
	}
	h.logger.InfoContext(c.Request().Context(), "mock permission schema write received",
		"payload", request,
		"relation_count", len(request.Relations),
	)
	return c.NoContent(http.StatusOK)
}
```

- [ ] **Step 8: Run mock handler tests**

Run:

```bash
go test ./internal/mock-permission-api/handlers -v
```

Expected: PASS.

- [ ] **Step 9: Write failing mock command health test**

Create `cmd/mock-permission-api/main_test.go`:

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRegisterHealthRoutes(t *testing.T) {
	e := echo.New()
	registerHealthRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/liveness", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
```

- [ ] **Step 10: Run mock command test and confirm failure**

Run:

```bash
go test ./cmd/mock-permission-api -v
```

Expected: FAIL because `cmd/mock-permission-api` does not exist.

- [ ] **Step 11: Add mock permission API command**

Create `cmd/mock-permission-api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/labstack/echo/v5"

	"github.com/hao0731/workspace-permission-management/internal/mock-permission-api/config"
	"github.com/hao0731/workspace-permission-management/internal/mock-permission-api/handlers"
	"github.com/hao0731/workspace-permission-management/internal/shared/health"
	sharedlogger "github.com/hao0731/workspace-permission-management/internal/shared/logger"
)

type processIndicator struct{}

func (processIndicator) Name() string {
	return "process"
}

func (processIndicator) IsHealthy(context.Context) bool {
	return true
}

func main() {
	if err := run(); err != nil {
		slog.Error("mock permission API stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := sharedlogger.New(cfg.Environment)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	e := echo.New()
	registerHealthRoutes(e)
	handlers.RegisterRoutes(e, handlers.NewSchemaHandler(logger))

	startConfig := echo.StartConfig{
		Address:         cfg.HTTPAddr,
		GracefulTimeout: cfg.ShutdownTimeout,
	}
	if err := startConfig.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func registerHealthRoutes(e *echo.Echo) {
	health.NewHealthManager(processIndicator{}).RegisterRoutes(e)
}
```

- [ ] **Step 12: Run mock service tests**

Run:

```bash
go test ./internal/mock-permission-api/... ./cmd/mock-permission-api -v
```

Expected: PASS.

- [ ] **Step 13: Commit mock permission API service**

Run:

```bash
git add internal/mock-permission-api cmd/mock-permission-api
git commit -m "feat: add mock permission API"
```

Expected: commit succeeds and includes only mock permission API service files.

---

### Task 4: Local Env, Docker Compose, And REST Client Example

**Files:**

- Modify: `.env`
- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Create: `examples/api/mock_permission_api.http`

- [ ] **Step 1: Add permission API environment values**

In `.env` and `.env.example`, add these values after `FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT=20` and before the function-service shutdown section:

```env
FUNCTION_SERVICE_PERMISSION_API_BASE_URL=http://localhost:8086
FUNCTION_SERVICE_PERMISSION_API_KEY=dev-permission-api-key
FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER=X-API-Key
```

In `.env` and `.env.example`, add this section after the Mock HR section:

```env
# Mock permission API
MOCK_PERMISSION_API_ENV=development
MOCK_PERMISSION_API_HTTP_ADDR=:8086
MOCK_PERMISSION_API_SHUTDOWN_TIMEOUT=10s
```

- [ ] **Step 2: Add Docker Compose service**

In `docker-compose.yml`, add `mock-permission-api` before `group-expiry-scheduler`:

```yaml
  mock-permission-api:
    image: golang:1.25
    container_name: workspace-permission-management-mock-permission-api
    working_dir: /workspace
    command: ["go", "run", "./cmd/mock-permission-api"]
    volumes:
      - .:/workspace
      - go_mod_cache:/go/pkg/mod
    environment:
      MOCK_PERMISSION_API_ENV: development
      MOCK_PERMISSION_API_HTTP_ADDR: :8086
      MOCK_PERMISSION_API_SHUTDOWN_TIMEOUT: 10s
    ports:
      - "8086:8086"
    networks:
      - workspace_permission_management
```

Do not add `function-service` to `docker-compose.yml`. Local function-service runs should use `.env` with `FUNCTION_SERVICE_PERMISSION_API_BASE_URL=http://localhost:8086`.

- [ ] **Step 3: Add REST Client examples**

Create `examples/api/mock_permission_api.http`:

```http
@baseUrl = http://localhost:8086
@apiKey = dev-permission-api-key

### Register schema resource attributes
POST {{baseUrl}}/api/v1/schema/write
Content-Type: application/json
X-API-Key: {{apiKey}}

{
  "definition": "todo",
  "relations": [
    {
      "resAttr": "can_edit_private_repo",
      "condition": "enable_dynamic_context",
      "isPublic": false
    },
    {
      "resAttr": "can_view_public_repo",
      "condition": "enable_dynamic_context",
      "isPublic": false
    }
  ]
}

### Malformed payload returns 400
POST {{baseUrl}}/api/v1/schema/write
Content-Type: application/json
X-API-Key: {{apiKey}}

{
```

- [ ] **Step 4: Validate Docker Compose config**

Run:

```bash
docker compose config >/tmp/workspace-permission-management-compose.yml
```

Expected: command exits 0 and renders a compose config that includes `mock-permission-api`.

If Docker is not available in the execution environment, record that this command was skipped and run this text-only check instead:

```bash
rg -n "mock-permission-api|FUNCTION_SERVICE_PERMISSION_API_BASE_URL|cmd/mock-permission-api" docker-compose.yml .env .env.example examples/api/mock_permission_api.http
```

Expected: all four files contain the new permission API references.

- [ ] **Step 5: Run env/config tests**

Run:

```bash
go test ./internal/function-service/config ./internal/mock-permission-api/config -v
```

Expected: PASS.

- [ ] **Step 6: Commit local development assets**

Run:

```bash
git add .env .env.example docker-compose.yml examples/api/mock_permission_api.http
git commit -m "chore: add permission API local development wiring"
```

Expected: commit succeeds and includes only env, compose, and REST Client example files.

---

### Task 5: Full Verification And Plan Completion

**Files:**

- Move: `docs/plans/active/2026-05-19-permission-api-client.md` to `docs/plans/completed/2026-05-19-permission-api-client.md`

- [ ] **Step 1: Run gofmt on touched Go files**

Run:

```bash
gofmt -w internal/shared/interactions/permission/api internal/function-service/config cmd/function-service internal/mock-permission-api cmd/mock-permission-api
```

Expected: command exits 0.

- [ ] **Step 2: Run targeted verification**

Run:

```bash
go test ./internal/shared/interactions/permission/... ./internal/function-service/... ./internal/mock-permission-api/... ./cmd/function-service ./cmd/mock-permission-api -v
```

Expected: PASS.

- [ ] **Step 3: Run full repository tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run diff hygiene check**

Run:

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 5: Move the completed plan**

Run:

```bash
git mv docs/plans/active/2026-05-19-permission-api-client.md docs/plans/completed/2026-05-19-permission-api-client.md
```

Expected: plan file is staged as a rename from `active` to `completed`.

- [ ] **Step 6: Commit plan completion**

Run:

```bash
git add docs/plans/completed/2026-05-19-permission-api-client.md
git commit -m "docs: complete permission API client plan"
```

Expected: commit succeeds and records the plan lifecycle transition.

## Self-Review

- Spec coverage: covered API client package, request DTOs, error DTOs, headers, function-service config and wiring, mock permission API, env files, Docker Compose, REST Client examples, and final verification.
- Incomplete-marker scan: no incomplete markers remain in this plan.
- Type consistency: `PermissionAPIConfig`, `permissionapi.New`, `RegisterResourceAttributesRequest`, `RegisterResourceAttributesRelationRequest`, `ErrorResponse`, `Error`, `NewSchemaHandler`, and `RegisterRoutes` are named consistently across tasks.
