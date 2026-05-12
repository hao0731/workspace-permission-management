# Workspace Creation Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `workspace-service`, `mock-hr`, `mock-function`, and the shared HR client boundary so `POST /api/v1/workspaces` persists workspaces, resolves owner display names, and best-effort publishes resource-create commands.

**Architecture:** Add three backend service entrypoints that follow the repository's existing Echo, MongoDB, NATS JetStream, CloudEvents, `slog`, and `viper` patterns. Keep domain packages free of framework and infrastructure dependencies, keep handlers thin, and keep HR HTTP calls plus JetStream publish and consume behavior behind explicit interfaces.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go Driver v2, NATS JetStream through `internal/shared/eventbus`, CloudEvents SDK for Go, `log/slog`, `viper`, standard `testing`, REST Client `.http` examples, Docker Compose local infrastructure.

---

## Source Designs

- [Workspace Service Design](../../designs/workspace-service.md)
- [Workspace Service API Design](../../designs/workspace-service-api-design.md)
- [Workspace Service Command Design](../../designs/workspace-service-command-design.md)
- [Mock HR Design](../../designs/mock-hr.md)
- [Mock Function Design](../../designs/mock-function.md)
- [Function Service Design](../../designs/function-service.md)

## Policy Summary

Task classification: Backend and Design-Plan Docs.

Policies followed:

- [Backend Architecture Principle](../../policies/backend-architecture-principle.md): handlers stay thin; domain and service logic stay free of Echo, MongoDB, NATS, and JetStream types; API and event schemas are explicit contracts; side effects are behind infrastructure boundaries.
- [Design and Plan Docs Policy](../../policies/design-and-plan-docs-policy.md): this implementation plan is stored under `docs/plans/active/` and links to its source design documents.

## Scope Check

This plan covers three services plus one shared interaction package because they form one integration slice:

- `mock-hr` supplies deterministic owner profile data.
- `workspace-service` depends on the shared HR client, persists workspace records, and publishes resource-create commands.
- `mock-function` consumes those commands and publishes resource upsert events.

The plan intentionally does not modify `function-service` to consume multiple `app.<APP_NAME>.resource.upserted` subjects. Local NATS streams should accept those subjects so `mock-function` can publish successfully; expanding `function-service` to project multiple app subjects requires a separate design.

## File Structure

Create these shared HR files:

- `internal/domain/hr/errors.go`: HR domain errors.
- `internal/domain/hr/user.go`: HR user domain model.
- `internal/domain/hr/validation.go`: HR user validation and normalization.
- `internal/domain/hr/validation_test.go`: HR user validation tests.
- `internal/shared/interactions/hr/client.go`: shared HR client interface.
- `internal/shared/interactions/hr/poc/mock_hr_client.go`: HTTP client for `mock-hr`.
- `internal/shared/interactions/hr/poc/mock_hr_client_test.go`: client tests with `httptest`.

Create these `mock-hr` files:

- `cmd/mock-hr/main.go`: service entrypoint, health route, Echo startup, graceful shutdown.
- `cmd/mock-hr/main_test.go`: health route registration smoke test.
- `internal/mock-hr/config/config.go`: environment config.
- `internal/mock-hr/config/config_test.go`: config validation tests.
- `internal/mock-hr/handlers/user_handler.go`: HTTP handlers and route registration.
- `internal/mock-hr/handlers/user_handler_test.go`: HTTP handler tests.
- `internal/mock-hr/services/user_service.go`: deterministic user service.
- `internal/mock-hr/services/user_service_test.go`: service tests.
- `internal/mock-hr/transport/user_request.go`: batch request decode and mapping.
- `internal/mock-hr/transport/user_request_test.go`: request decode tests.
- `internal/mock-hr/transport/user_response.go`: response DTOs.
- `internal/mock-hr/transport/user_response_test.go`: response mapping tests.

Create these `workspace-service` files:

- `cmd/workspace-service/main.go`: service entrypoint, dependency wiring, health route, graceful shutdown.
- `cmd/workspace-service/main_test.go`: health route and eventbus config smoke tests.
- `cmd/workspace-service/resource_create_publisher.go`: concrete JetStream publisher wrapper.
- `cmd/workspace-service/resource_create_publisher_test.go`: publisher tests.
- `internal/domain/workspace/errors.go`: workspace domain errors.
- `internal/domain/workspace/workspace.go`: workspace models and command models.
- `internal/domain/workspace/validation.go`: workspace input and command validation.
- `internal/domain/workspace/validation_test.go`: domain validation tests.
- `internal/workspace-service/config/config.go`: environment config and resource mappings.
- `internal/workspace-service/config/config_test.go`: config validation tests.
- `internal/workspace-service/handlers/workspace_handler.go`: HTTP handler and route registration.
- `internal/workspace-service/handlers/workspace_handler_test.go`: HTTP handler tests.
- `internal/workspace-service/repositories/mongo_workspace_repository.go`: MongoDB insert and index behavior.
- `internal/workspace-service/repositories/mongo_workspace_repository_test.go`: repository tests.
- `internal/workspace-service/services/workspace_service.go`: create workflow.
- `internal/workspace-service/services/workspace_service_test.go`: workflow tests.
- `internal/workspace-service/transport/workspace_request.go`: request decode and mapping.
- `internal/workspace-service/transport/workspace_request_test.go`: request decode tests.
- `internal/workspace-service/transport/workspace_response.go`: response DTOs.
- `internal/workspace-service/transport/workspace_response_test.go`: response tests.
- `internal/workspace-service/transport/resource_create_event.go`: CloudEvent builder.
- `internal/workspace-service/transport/resource_create_event_test.go`: event builder tests.

Create these `mock-function` files:

- `cmd/mock-function/main.go`: service entrypoint, NATS wiring, health route, graceful shutdown.
- `cmd/mock-function/main_test.go`: eventbus config and health route tests.
- `cmd/mock-function/resource_upsert_publisher.go`: concrete JetStream publisher wrapper.
- `cmd/mock-function/resource_upsert_publisher_test.go`: publisher tests.
- `internal/domain/mockfunction/errors.go`: mock-function domain errors.
- `internal/domain/mockfunction/mockfunction.go`: command and upsert event models.
- `internal/domain/mockfunction/validation.go`: validation and normalization.
- `internal/domain/mockfunction/validation_test.go`: validation tests.
- `internal/mock-function/config/config.go`: environment config and subject derivation.
- `internal/mock-function/config/config_test.go`: config validation tests.
- `internal/mock-function/handlers/resource_create_event_handler.go`: JetStream handler.
- `internal/mock-function/handlers/resource_create_event_handler_test.go`: handler tests.
- `internal/mock-function/services/resource_service.go`: command handling workflow.
- `internal/mock-function/services/resource_service_test.go`: service tests.
- `internal/mock-function/transport/resource_create_event.go`: command parser.
- `internal/mock-function/transport/resource_create_event_test.go`: command parser tests.
- `internal/mock-function/transport/resource_upsert_event.go`: upsert event builder.
- `internal/mock-function/transport/resource_upsert_event_test.go`: upsert event tests.

Modify these local integration files:

- `.env.example`: add `mock-hr`, `workspace-service`, and `mock-function` config.
- `docker-compose.yml`: add local service containers and NATS stream or consumer setup for resource-create commands and mock upsert publishes.
- `examples/api/mock_hr.http`: mock HR REST examples.
- `examples/api/workspaces.http`: workspace REST examples.

## Task 1: Shared HR Domain and Client Boundary

**Files:**
- Create: `internal/domain/hr/errors.go`
- Create: `internal/domain/hr/user.go`
- Create: `internal/domain/hr/validation.go`
- Create: `internal/domain/hr/validation_test.go`
- Create: `internal/shared/interactions/hr/client.go`
- Create: `internal/shared/interactions/hr/poc/mock_hr_client.go`
- Create: `internal/shared/interactions/hr/poc/mock_hr_client_test.go`

- [x] **Step 1: Write failing HR domain validation tests**

Create `internal/domain/hr/validation_test.go`:

```go
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
```

- [x] **Step 2: Write failing POC HR client tests**

Create `internal/shared/interactions/hr/poc/mock_hr_client_test.go`:

```go
package poc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMockHRClientGet(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]string{
				"nt_account":   "user1",
				"display_name": "Test User 測試員",
			},
		})
	}))
	defer server.Close()

	client := New(server.URL)
	user, err := client.Get(context.Background(), "user1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if gotPath != "/api/v1/users/user1" {
		t.Fatalf("path = %q, want /api/v1/users/user1", gotPath)
	}
	if user.NTAccount != "user1" || user.DisplayName != "Test User 測試員" {
		t.Fatalf("user = %+v", user)
	}
}

func TestMockHRClientBatchGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user-list" {
			t.Fatalf("path = %q, want /api/v1/user-list", r.URL.Path)
		}
		var body struct {
			NTAccounts []string `json:"nt_accounts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.NTAccounts) != 2 || body.NTAccounts[0] != "user1" || body.NTAccounts[1] != "user2" {
			t.Fatalf("nt_accounts = %#v", body.NTAccounts)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"users": []map[string]string{
				{"nt_account": "user1", "display_name": "Test User 測試員"},
				{"nt_account": "user2", "display_name": "Test User 測試員"},
			},
		})
	}))
	defer server.Close()

	client := New(server.URL)
	users, err := client.BatchGet(context.Background(), []string{"user1", "user2"})
	if err != nil {
		t.Fatalf("BatchGet() error = %v", err)
	}
	if len(users) != 2 || users[1].NTAccount != "user2" {
		t.Fatalf("users = %+v", users)
	}
}

func TestMockHRClientRejectsInvalidInput(t *testing.T) {
	client := New("http://127.0.0.1:1")
	if _, err := client.Get(context.Background(), " "); err == nil {
		t.Fatal("Get() error = nil, want error")
	}
	if _, err := client.BatchGet(context.Background(), nil); err == nil {
		t.Fatal("BatchGet() nil error = nil, want error")
	}
	if _, err := client.BatchGet(context.Background(), []string{"user1", " "}); err == nil {
		t.Fatal("BatchGet() blank error = nil, want error")
	}
}

func TestMockHRClientReturnsErrorForNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(server.URL)
	if _, err := client.Get(context.Background(), "user1"); err == nil {
		t.Fatal("Get() error = nil, want error")
	}
}
```

- [x] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/domain/hr ./internal/shared/interactions/hr/poc
```

Expected: FAIL because the new packages and symbols are not implemented.

- [x] **Step 4: Implement HR domain and client interface**

Create `internal/domain/hr/errors.go`:

```go
package hr

import "errors"

var ErrInvalidInput = errors.New("invalid hr input")
```

Create `internal/domain/hr/user.go`:

```go
package hr

type User struct {
	NTAccount   string
	DisplayName string
}
```

Create `internal/domain/hr/validation.go`:

```go
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
```

Create `internal/shared/interactions/hr/client.go`:

```go
package hr

import (
	"context"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
)

type Client interface {
	Get(ctx context.Context, ntAccount string) (domainhr.User, error)
	BatchGet(ctx context.Context, ntAccounts []string) ([]domainhr.User, error)
}
```

- [x] **Step 5: Implement POC mock HR client**

Create `internal/shared/interactions/hr/poc/mock_hr_client.go`:

```go
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

func (c *Client) do(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send hr request: %w", err)
	}
	defer resp.Body.Close()
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
```

- [x] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/domain/hr ./internal/shared/interactions/hr ./internal/shared/interactions/hr/poc
```

Expected: PASS.

Commit:

```bash
git add internal/domain/hr internal/shared/interactions/hr
git commit -m "feat: add shared HR client boundary"
```

## Task 2: Mock HR Service

**Files:**
- Create: `internal/mock-hr/config/config.go`
- Create: `internal/mock-hr/config/config_test.go`
- Create: `internal/mock-hr/services/user_service.go`
- Create: `internal/mock-hr/services/user_service_test.go`
- Create: `internal/mock-hr/transport/user_request.go`
- Create: `internal/mock-hr/transport/user_request_test.go`
- Create: `internal/mock-hr/transport/user_response.go`
- Create: `internal/mock-hr/transport/user_response_test.go`
- Create: `internal/mock-hr/handlers/user_handler.go`
- Create: `internal/mock-hr/handlers/user_handler_test.go`
- Create: `cmd/mock-hr/main.go`
- Create: `cmd/mock-hr/main_test.go`
- Create: `examples/api/mock_hr.http`

- [ ] **Step 1: Write failing service and transport tests**

Create tests that assert deterministic mock HR behavior:

```go
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
```

Create request tests for `DecodeUserListRequest`:

```go
func TestDecodeUserListRequestRejectsEmptyList(t *testing.T) {
	_, err := DecodeUserListRequest(strings.NewReader(`{"nt_accounts":[]}`))
	if err == nil {
		t.Fatal("DecodeUserListRequest() error = nil, want error")
	}
}
```

- [ ] **Step 2: Write failing handler and config tests**

Add handler tests covering:

```go
func TestGetUserReturnsFixedDisplayName(t *testing.T) {
	e := echo.New()
	handler := NewUserHandler(services.NewUserService(), slog.Default())
	RegisterRoutes(e, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"display_name":"Test User 測試員"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
```

Add config tests covering required HTTP address and positive shutdown timeout:

```go
func TestConfigValidateRequiresHTTPAddr(t *testing.T) {
	cfg := Config{Environment: environment.Development, ShutdownTimeout: time.Second}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/mock-hr/... ./cmd/mock-hr
```

Expected: FAIL because `mock-hr` packages are not implemented.

- [ ] **Step 4: Implement mock HR service packages**

Implement `internal/mock-hr/services/user_service.go`:

```go
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
```

Implement transport DTOs with these signatures:

```go
func DecodeUserListRequest(body io.Reader) (UserListRequest, error)
func (request UserListRequest) ToDomain() ([]string, error)
func NewUserResponse(user domainhr.User) UserResponse
func NewUserListResponse(users []domainhr.User) UserListResponse
```

Implement handlers with these routes:

```go
func RegisterRoutes(e *echo.Echo, handler *UserHandler) {
	e.GET("/api/v1/users/:nt_account", handler.GetUser)
	e.POST("/api/v1/user-list", handler.BatchGetUsers)
}
```

Map `domainhr.ErrInvalidInput` to `400 validation_failed`; map unexpected errors to `500 internal_error`.

- [ ] **Step 5: Implement config, main, and REST examples**

Implement `internal/mock-hr/config/config.go` following the existing config pattern:

```go
type Config struct {
	Environment     environment.Environment
	HTTPAddr        string
	ShutdownTimeout time.Duration
}
```

Defaults:

```go
v.SetDefault("MOCK_HR_ENV", string(environment.Development))
v.SetDefault("MOCK_HR_SHUTDOWN_TIMEOUT", "10s")
```

Implement `cmd/mock-hr/main.go` with:

- `processIndicator` matching `cmd/group-service/main.go`.
- `config.Load()`.
- `sharedlogger.New(cfg.Environment)`.
- `health.NewHealthManager(processIndicator{}).RegisterRoutes(e)`.
- `handlers.RegisterRoutes(e, handlers.NewUserHandler(services.NewUserService(), logger))`.
- `echo.StartConfig{Address: cfg.HTTPAddr, GracefulTimeout: cfg.ShutdownTimeout}`.

Create `examples/api/mock_hr.http`:

```http
@baseUrl = http://localhost:8082

### Get user
GET {{baseUrl}}/api/v1/users/user1

### Batch get users
POST {{baseUrl}}/api/v1/user-list
Content-Type: application/json

{
  "nt_accounts": ["user1", "user2", "user1"]
}

### Empty user list returns 400
POST {{baseUrl}}/api/v1/user-list
Content-Type: application/json

{
  "nt_accounts": []
}

### Empty account returns 400
POST {{baseUrl}}/api/v1/user-list
Content-Type: application/json

{
  "nt_accounts": ["user1", ""]
}
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/domain/hr ./internal/mock-hr/... ./internal/shared/interactions/hr/... ./cmd/mock-hr
```

Expected: PASS.

Commit:

```bash
git add internal/mock-hr cmd/mock-hr examples/api/mock_hr.http
git commit -m "feat: add mock HR service"
```

## Task 3: Workspace Domain and Transport Contracts

**Files:**
- Create: `internal/domain/workspace/errors.go`
- Create: `internal/domain/workspace/workspace.go`
- Create: `internal/domain/workspace/validation.go`
- Create: `internal/domain/workspace/validation_test.go`
- Create: `internal/workspace-service/transport/workspace_request.go`
- Create: `internal/workspace-service/transport/workspace_request_test.go`
- Create: `internal/workspace-service/transport/workspace_response.go`
- Create: `internal/workspace-service/transport/workspace_response_test.go`
- Create: `internal/workspace-service/transport/resource_create_event.go`
- Create: `internal/workspace-service/transport/resource_create_event_test.go`

- [ ] **Step 1: Write failing workspace domain tests**

Create tests for request validation and normalization:

```go
func TestCreateInputValidateRejectsRequiredFields(t *testing.T) {
	tests := []CreateInput{
		{Description: "desc", OwnerNTAccount: "user1"},
		{Name: "name", OwnerNTAccount: "user1"},
		{Name: "name", Description: "desc"},
		{Name: "name", Description: "desc", OwnerNTAccount: "user1", Documents: &ResourceRequest{}},
	}
	for _, input := range tests {
		if err := input.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate() error = %v, want ErrInvalidInput", err)
		}
	}
}

func TestCreateInputNormalize(t *testing.T) {
	input := CreateInput{
		Name:           " Project ",
		Description:    " Description ",
		OwnerNTAccount: " user1 ",
		Documents:      &ResourceRequest{ResourceName: " Docs "},
	}
	normalized := input.Normalize()
	if normalized.Name != "Project" || normalized.OwnerNTAccount != "user1" || normalized.Documents.ResourceName != "Docs" {
		t.Fatalf("Normalize() = %+v", normalized)
	}
}
```

Create tests for `ResourceCreateCommand.Validate()` requiring workspace ID, app name, resource type, resource name, event ID, and event time.

- [ ] **Step 2: Write failing transport tests**

Add request tests:

```go
func TestDecodeWorkspaceCreateRequestWithAllResources(t *testing.T) {
	body := strings.NewReader(`{
		"name":" Workspace ",
		"description":" Description ",
		"owner":" user1 ",
		"documents":{"resource_name":" Docs "},
		"tasks":{"resource_name":" Tasks "},
		"drive":{"resource_name":" Drive "}
	}`)
	request, err := DecodeWorkspaceCreateRequest(body)
	if err != nil {
		t.Fatalf("DecodeWorkspaceCreateRequest() error = %v", err)
	}
	input, err := request.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain() error = %v", err)
	}
	if input.Documents == nil || input.Tasks == nil || input.Drive == nil {
		t.Fatalf("resources = documents:%v tasks:%v drive:%v", input.Documents, input.Tasks, input.Drive)
	}
}
```

Add response tests asserting:

- `workspace.id`, `name`, `description` are present.
- `owner.nt_account` and `owner.display_name` are present.
- `owner_nt_account`, `created_at`, and `updated_at` are not present.

Add event builder test:

```go
func TestNewResourceCreateEvent(t *testing.T) {
	eventTime := time.Date(2026, 5, 5, 7, 31, 0, 0, time.UTC)
	data, err := NewResourceCreateEvent(workspace.ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
		EventID:      "event-1",
		EventTime:    eventTime,
	})
	if err != nil {
		t.Fatalf("NewResourceCreateEvent() error = %v", err)
	}
	var event cloudevents.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event.Type() != "cmd.app.documents.resource.create" || event.Subject() != "workspace-1" {
		t.Fatalf("type=%q subject=%q", event.Type(), event.Subject())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/domain/workspace ./internal/workspace-service/transport
```

Expected: FAIL because packages are not implemented.

- [ ] **Step 4: Implement workspace domain package**

Create `internal/domain/workspace/workspace.go` with these models:

```go
package workspace

import "time"

type Workspace struct {
	ID             string
	Name           string
	Description    string
	OwnerNTAccount string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ResourceSection string

const (
	ResourceSectionDocuments ResourceSection = "documents"
	ResourceSectionTasks     ResourceSection = "tasks"
	ResourceSectionDrive     ResourceSection = "drive"
)

type ResourceRequest struct {
	ResourceName string
}

type CreateInput struct {
	Name           string
	Description    string
	OwnerNTAccount string
	Documents      *ResourceRequest
	Tasks          *ResourceRequest
	Drive           *ResourceRequest
}

type ResourceCreateCommand struct {
	WorkspaceID  string
	Section      ResourceSection
	AppName      string
	ResourceName string
	ResourceType string
	EventID      string
	EventTime    time.Time
}

func (c ResourceCreateCommand) Subject() string {
	return "cmd.app." + c.AppName + ".resource.create"
}
```

Create `errors.go` with `ErrInvalidInput`.

Create `validation.go` with `Normalize()` and `Validate()` methods for `CreateInput`, `ResourceRequest`, `Workspace`, and `ResourceCreateCommand`.

- [ ] **Step 5: Implement workspace transport package**

Implement request decode:

```go
type WorkspaceCreateRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Owner       string           `json:"owner"`
	Documents   *ResourceRequest `json:"documents"`
	Tasks       *ResourceRequest `json:"tasks"`
	Drive       *ResourceRequest `json:"drive"`
}

type ResourceRequest struct {
	ResourceName string `json:"resource_name"`
}
```

Implement response DTO:

```go
type WorkspaceCreateResponse struct {
	Workspace WorkspaceResponse `json:"workspace"`
}

type WorkspaceResponse struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Owner       OwnerResponse `json:"owner"`
}

type OwnerResponse struct {
	NTAccount   string `json:"nt_account"`
	DisplayName string `json:"display_name"`
}
```

Implement command event builder with CloudEvents SDK:

```go
func NewResourceCreateEvent(command workspace.ResourceCreateCommand) ([]byte, error) {
	if err := command.Validate(); err != nil {
		return nil, err
	}
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType(command.Subject())
	event.SetSource("workspace-service")
	event.SetSubject(command.WorkspaceID)
	event.SetID(command.EventID)
	event.SetTime(command.EventTime)
	if err := event.SetData(cloudevents.ApplicationJSON, resourceCreateData{
		WorkspaceID:  command.WorkspaceID,
		ResourceName: command.ResourceName,
		ResourceType: command.ResourceType,
	}); err != nil {
		return nil, fmt.Errorf("set resource create event data: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal resource create event: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/domain/workspace ./internal/workspace-service/transport
```

Expected: PASS.

Commit:

```bash
git add internal/domain/workspace internal/workspace-service/transport
git commit -m "feat: add workspace domain and transport contracts"
```

## Task 4: Workspace Repository and Service Workflow

**Files:**
- Create: `internal/workspace-service/repositories/mongo_workspace_repository.go`
- Create: `internal/workspace-service/repositories/mongo_workspace_repository_test.go`
- Create: `internal/workspace-service/services/workspace_service.go`
- Create: `internal/workspace-service/services/workspace_service_test.go`

- [ ] **Step 1: Write failing repository tests**

Follow existing Mongo repository test setup patterns from `internal/function-service/repositories/mongo_resource_repository_test.go`.

Test insert behavior:

```go
func TestMongoWorkspaceRepositoryCreate(t *testing.T) {
	db := setupWorkspaceTestDB(t)
	repo := NewMongoWorkspaceRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	created, err := repo.Create(ctx, workspace.Workspace{
		ID:             "workspace-1",
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.OwnerNTAccount != "user1" {
		t.Fatalf("OwnerNTAccount = %q", created.OwnerNTAccount)
	}

	var doc bson.M
	if err := db.Collection("workspaces").FindOne(ctx, bson.M{"_id": "workspace-1"}).Decode(&doc); err != nil {
		t.Fatalf("find workspace: %v", err)
	}
	if _, ok := doc["display_name"]; ok {
		t.Fatal("display_name was persisted, want omitted")
	}
}
```

Test `EnsureIndexes` creates the `owner_nt_account + created_at + _id` index.

- [ ] **Step 2: Write failing service workflow tests**

Use fakes for repository, HR client, publisher, clock, ID generator, and logger.

Test successful create:

```go
func TestWorkspaceServiceCreateWorkspace(t *testing.T) {
	repo := &fakeWorkspaceRepository{}
	hrClient := &fakeHRClient{user: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"}}
	publisher := &fakeCommandPublisher{}
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	service := NewWorkspaceService(repo, hrClient, publisher,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequenceIDs("workspace-1", "event-1", "event-2", "event-3")),
		WithResourceMappings(ResourceMappings{
			Documents: ResourceMapping{AppName: "documents", ResourceType: "document"},
			Tasks:     ResourceMapping{AppName: "tasks", ResourceType: "task"},
			Drive:     ResourceMapping{AppName: "drive", ResourceType: "file"},
		}),
	)

	result, err := service.CreateWorkspace(context.Background(), workspace.CreateInput{
		Name:           "Planning",
		Description:    "Planning workspace",
		OwnerNTAccount: "user1",
		Documents:      &workspace.ResourceRequest{ResourceName: "Docs"},
		Tasks:          &workspace.ResourceRequest{ResourceName: "Tasks"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	if result.Workspace.ID != "workspace-1" || result.Owner.DisplayName != "Test User 測試員" {
		t.Fatalf("result = %+v", result)
	}
	if len(publisher.commands) != 2 {
		t.Fatalf("commands = %+v, want 2", publisher.commands)
	}
}
```

Also test:

- HR failure inserts no document and publishes no commands.
- Repository failure publishes no commands.
- Publisher failure returns success and attempts later commands.
- Commands are attempted in `documents`, `tasks`, `drive` order.

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/workspace-service/repositories ./internal/workspace-service/services
```

Expected: FAIL because repository and service are not implemented.

- [ ] **Step 4: Implement Mongo workspace repository**

Implement `internal/workspace-service/repositories/mongo_workspace_repository.go`:

```go
const workspaceCollectionName = "workspaces"

type MongoWorkspaceRepository struct {
	collection *mongo.Collection
}

func NewMongoWorkspaceRepository(db *mongo.Database) *MongoWorkspaceRepository {
	return &MongoWorkspaceRepository{collection: db.Collection(workspaceCollectionName)}
}

func (r *MongoWorkspaceRepository) EnsureIndexes(ctx context.Context) error {
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "owner_nt_account", Value: 1},
			{Key: "created_at", Value: -1},
			{Key: "_id", Value: -1},
		},
	}
	if _, err := r.collection.Indexes().CreateOne(ctx, model); err != nil {
		return fmt.Errorf("create workspaces index: %w", err)
	}
	return nil
}

func (r *MongoWorkspaceRepository) Create(ctx context.Context, input workspace.Workspace) (workspace.Workspace, error) {
	if err := input.Validate(); err != nil {
		return workspace.Workspace{}, err
	}
	doc := workspaceDocument{
		ID:             input.ID,
		Name:           input.Name,
		Description:    input.Description,
		OwnerNTAccount: input.OwnerNTAccount,
		CreatedAt:      input.CreatedAt,
		UpdatedAt:      input.UpdatedAt,
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return workspace.Workspace{}, fmt.Errorf("insert workspace: %w", err)
	}
	return doc.toDomain(), nil
}
```

- [ ] **Step 5: Implement workspace service**

Implement these service types:

```go
type WorkspaceRepository interface {
	Create(ctx context.Context, input workspace.Workspace) (workspace.Workspace, error)
}

type ResourceCreateCommandPublisher interface {
	PublishResourceCreateCommand(ctx context.Context, command workspace.ResourceCreateCommand) error
}

type CreateWorkspaceResult struct {
	Workspace workspace.Workspace
	Owner     domainhr.User
}

type ResourceMapping struct {
	AppName      string
	ResourceType string
}

type ResourceMappings struct {
	Documents ResourceMapping
	Tasks     ResourceMapping
	Drive     ResourceMapping
}
```

Implement `CreateWorkspace` with this behavior:

1. Normalize and validate input.
2. Call `hrClient.Get(ctx, input.OwnerNTAccount)`.
3. Return `ErrHRLookupFailed` wrapping the HR error when lookup fails.
4. Generate workspace ID using the first ID call.
5. Insert `workspace.Workspace` with `OwnerNTAccount` only.
6. Build commands for present resource sections.
7. Generate one event ID per command.
8. Publish each command and log failures without returning an error.
9. Return the created workspace and HR user.

Define `ErrHRLookupFailed` in the services package:

```go
var ErrHRLookupFailed = errors.New("hr lookup failed")
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/workspace-service/repositories ./internal/workspace-service/services
```

Expected: PASS.

Commit:

```bash
git add internal/workspace-service/repositories internal/workspace-service/services
git commit -m "feat: add workspace persistence workflow"
```

## Task 5: Workspace HTTP API, Config, Main Wiring, and Publisher

**Files:**
- Create: `internal/workspace-service/config/config.go`
- Create: `internal/workspace-service/config/config_test.go`
- Create: `internal/workspace-service/handlers/workspace_handler.go`
- Create: `internal/workspace-service/handlers/workspace_handler_test.go`
- Create: `cmd/workspace-service/resource_create_publisher.go`
- Create: `cmd/workspace-service/resource_create_publisher_test.go`
- Create: `cmd/workspace-service/main.go`
- Create: `cmd/workspace-service/main_test.go`
- Create: `examples/api/workspaces.http`

- [ ] **Step 1: Write failing config and handler tests**

Config tests must cover:

- Required HTTP address, MongoDB URI, MongoDB database, NATS URL, HR base URL, app names, and resource types.
- Positive shutdown timeout.
- Positive command publish timeout.
- Invalid app names with dots or whitespace.
- Valid mappings for documents, tasks, and drive.

Handler test for success:

```go
func TestWorkspaceHandlerCreateWorkspace(t *testing.T) {
	e := echo.New()
	service := &fakeHTTPWorkspaceService{
		result: services.CreateWorkspaceResult{
			Workspace: workspace.Workspace{
				ID:             "workspace-1",
				Name:           "Planning",
				Description:    "Planning workspace",
				OwnerNTAccount: "user1",
			},
			Owner: domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"},
		},
	}
	RegisterRoutes(e, NewWorkspaceHandler(service, slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{
		"name":"Planning",
		"description":"Planning workspace",
		"owner":"user1"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"display_name":"Test User 測試員"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
```

Handler tests must also cover:

- Malformed JSON returns `400 validation_failed`.
- Domain invalid input returns `400 validation_failed`.
- `services.ErrHRLookupFailed` returns `502 hr_lookup_failed`.
- Unexpected service error returns `500 internal_error`.

- [ ] **Step 2: Write failing publisher and main tests**

Publisher test:

```go
func TestResourceCreatePublisherPublishesExpectedSubject(t *testing.T) {
	publisher := &fakeMessagePublisher{}
	resourcePublisher := newResourceCreatePublisher(publisher, eventbus.WithPublishTimeout(time.Second))
	err := resourcePublisher.PublishResourceCreateCommand(context.Background(), workspace.ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		Section:      workspace.ResourceSectionDocuments,
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PublishResourceCreateCommand() error = %v", err)
	}
	if publisher.subject != "cmd.app.documents.resource.create" {
		t.Fatalf("subject = %q", publisher.subject)
	}
}
```

Main tests should verify `registerHealthRoutes` serves `/health/liveness`.

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/workspace-service/config ./internal/workspace-service/handlers ./cmd/workspace-service
```

Expected: FAIL because config, handler, publisher, and main wiring are not implemented.

- [ ] **Step 4: Implement workspace config**

Implement `internal/workspace-service/config/config.go` with:

```go
type Config struct {
	Environment           environment.Environment
	HTTPAddr              string
	MongoDB               MongoDBConfig
	NATS                  NATSConfig
	HR                    HRConfig
	ResourceMappings      ResourceMappings
	CommandPublishTimeout time.Duration
	ShutdownTimeout       time.Duration
}
```

Use these environment keys:

- `WORKSPACE_SERVICE_ENV`
- `WORKSPACE_SERVICE_HTTP_ADDR`
- `WORKSPACE_SERVICE_MONGODB_URI`
- `WORKSPACE_SERVICE_MONGODB_DATABASE`
- `WORKSPACE_SERVICE_NATS_URL`
- `WORKSPACE_SERVICE_HR_BASE_URL`
- `WORKSPACE_SERVICE_DOCUMENTS_APP_NAME`
- `WORKSPACE_SERVICE_DOCUMENTS_RESOURCE_TYPE`
- `WORKSPACE_SERVICE_TASKS_APP_NAME`
- `WORKSPACE_SERVICE_TASKS_RESOURCE_TYPE`
- `WORKSPACE_SERVICE_DRIVE_APP_NAME`
- `WORKSPACE_SERVICE_DRIVE_RESOURCE_TYPE`
- `WORKSPACE_SERVICE_COMMAND_PUBLISH_TIMEOUT`
- `WORKSPACE_SERVICE_SHUTDOWN_TIMEOUT`

Add helper:

```go
func isValidSubjectToken(value string) bool {
	return strings.TrimSpace(value) != "" &&
		!strings.ContainsAny(value, ". \t\r\n")
}
```

- [ ] **Step 5: Implement handler, publisher, main, and examples**

Implement handler route:

```go
func RegisterRoutes(e *echo.Echo, handler *WorkspaceHandler) {
	e.POST("/api/v1/workspaces", handler.CreateWorkspace)
}
```

Implement `CreateWorkspace`:

- Decode request with `transport.DecodeWorkspaceCreateRequest`.
- Map to domain with `request.ToDomain()`.
- Call service.
- Map `workspace.ErrInvalidInput` to `400`.
- Map `services.ErrHRLookupFailed` to `502`.
- Render `transport.NewWorkspaceCreateResponse(result.Workspace, result.Owner)` with `201`.

Implement `cmd/workspace-service/resource_create_publisher.go` following `cmd/function-service/resource_deleted_publisher.go`.

Implement `cmd/workspace-service/main.go`:

- Load config.
- Create logger.
- Connect MongoDB and ensure indexes.
- Connect NATS and create `eventbus.NewJetStreamProducer`.
- Create POC HR client with `poc.New(cfg.HR.BaseURL)`.
- Create resource publisher with configured publish timeout.
- Create service with repository, HR client, publisher, mappings, logger.
- Register health and workspace routes.
- Start Echo with graceful shutdown.

Create `examples/api/workspaces.http` using the examples from [Workspace Service API Design](../../designs/workspace-service-api-design.md#rest-client-examples), with `@baseUrl = http://localhost:8083`.

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/domain/workspace ./internal/workspace-service/... ./cmd/workspace-service
```

Expected: PASS.

Commit:

```bash
git add internal/workspace-service cmd/workspace-service examples/api/workspaces.http
git commit -m "feat: add workspace service API"
```

## Task 6: Mock Function Domain, Transport, Service, and Handler

**Files:**
- Create: `internal/domain/mockfunction/errors.go`
- Create: `internal/domain/mockfunction/mockfunction.go`
- Create: `internal/domain/mockfunction/validation.go`
- Create: `internal/domain/mockfunction/validation_test.go`
- Create: `internal/mock-function/transport/resource_create_event.go`
- Create: `internal/mock-function/transport/resource_create_event_test.go`
- Create: `internal/mock-function/transport/resource_upsert_event.go`
- Create: `internal/mock-function/transport/resource_upsert_event_test.go`
- Create: `internal/mock-function/services/resource_service.go`
- Create: `internal/mock-function/services/resource_service_test.go`
- Create: `internal/mock-function/handlers/resource_create_event_handler.go`
- Create: `internal/mock-function/handlers/resource_create_event_handler_test.go`

- [ ] **Step 1: Write failing domain and transport tests**

Domain tests must cover:

- Create command rejects empty workspace ID, app name, resource name, and resource type.
- Upsert event rejects empty resource ID, app name, display name, resource type, workspace ID, event ID, and event time.
- App name normalization trims whitespace.

Command parser test:

```go
func TestParseResourceCreateCommandEvent(t *testing.T) {
	event := cloudevents.NewEvent()
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetType("cmd.app.documents.resource.create")
	event.SetSource("workspace-service")
	event.SetSubject("workspace-1")
	event.SetID("event-1")
	event.SetTime(time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC))
	if err := event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"workspace_id":  "workspace-1",
		"resource_name": "Docs",
		"resource_type": "document",
	}); err != nil {
		t.Fatalf("SetData() error = %v", err)
	}
	data, _ := json.Marshal(event)

	command, err := ParseResourceCreateCommandEvent(data, "cmd.app.documents.resource.create", map[string]string{
		"cmd.app.documents.resource.create": "documents",
	})
	if err != nil {
		t.Fatalf("ParseResourceCreateCommandEvent() error = %v", err)
	}
	if command.AppName != "documents" || command.WorkspaceID != "workspace-1" {
		t.Fatalf("command = %+v", command)
	}
}
```

Upsert builder test must assert `type = app.documents.resource.upserted`, `source = mock-function`, `subject = resource_id`, `function_key = documents`, and `resource_tags = []`.

- [ ] **Step 2: Write failing service and handler tests**

Service test:

```go
func TestResourceServiceHandleCreateCommandPublishesUpsert(t *testing.T) {
	publisher := &fakeUpsertPublisher{}
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	service := NewResourceService(publisher,
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequenceIDs("resource-1", "event-1")),
	)
	err := service.HandleResourceCreate(context.Background(), mockfunction.ResourceCreateCommand{
		WorkspaceID:  "workspace-1",
		AppName:      "documents",
		ResourceName: "Docs",
		ResourceType: "document",
	})
	if err != nil {
		t.Fatalf("HandleResourceCreate() error = %v", err)
	}
	if len(publisher.events) != 1 || publisher.events[0].FunctionKey != "documents" {
		t.Fatalf("events = %+v", publisher.events)
	}
}
```

Handler tests must cover:

- Valid message returns `eventbus.HandleResultAck`.
- Malformed JSON returns `eventbus.HandleResultTerminate`.
- Unknown subject returns `eventbus.HandleResultTerminate`.
- Publisher failure returns `eventbus.HandleResultRetry`.

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/domain/mockfunction ./internal/mock-function/transport ./internal/mock-function/services ./internal/mock-function/handlers
```

Expected: FAIL because packages are not implemented.

- [ ] **Step 4: Implement domain and transport**

Create domain models:

```go
type ResourceCreateCommand struct {
	WorkspaceID  string
	AppName      string
	ResourceName string
	ResourceType string
}

type ResourceUpsertEvent struct {
	ResourceID   string
	DisplayName  string
	ResourceType string
	ResourceTags []string
	FunctionKey  string
	WorkspaceID  string
	EventID      string
	EventTime    time.Time
}
```

Implement parser:

```go
func ParseResourceCreateCommandEvent(data []byte, messageSubject string, subjectAppNames map[string]string) (mockfunction.ResourceCreateCommand, error)
```

Validation rules:

- CloudEvent type must equal message subject.
- Message subject must exist in `subjectAppNames`.
- CloudEvent subject must equal `data.workspace_id`.
- Required data fields must be non-empty after trimming.

Implement upsert builder:

```go
func NewResourceUpsertEvent(event mockfunction.ResourceUpsertEvent) ([]byte, string, error)
```

Return both CloudEvent bytes and subject so the publisher can call `eventbus.Producer.Publish(ctx, subject, data)`.

- [ ] **Step 5: Implement service and handler**

Service dependencies:

```go
type ResourceUpsertPublisher interface {
	PublishResourceUpsert(ctx context.Context, event mockfunction.ResourceUpsertEvent) error
}
```

`HandleResourceCreate` behavior:

- Validate command.
- Log command receipt through the injected logger if the service stores a logger.
- Generate resource ID and event ID.
- Publish upsert event.
- Return publish error for retry.

Handler constructor:

```go
func NewResourceCreateEventHandler(service ResourceCreateService, subjectAppNames map[string]string, logger *slog.Logger) *ResourceCreateEventHandler
```

Handler behavior:

- Parse invalid command as terminate.
- Service invalid input as terminate.
- Service publish or unexpected errors as retry.
- Success as ack.

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/domain/mockfunction ./internal/mock-function/transport ./internal/mock-function/services ./internal/mock-function/handlers
```

Expected: PASS.

Commit:

```bash
git add internal/domain/mockfunction internal/mock-function
git commit -m "feat: add mock function event workflow"
```

## Task 7: Mock Function Config, Main Wiring, Publisher, and Local Infrastructure

**Files:**
- Create: `internal/mock-function/config/config.go`
- Create: `internal/mock-function/config/config_test.go`
- Create: `cmd/mock-function/resource_upsert_publisher.go`
- Create: `cmd/mock-function/resource_upsert_publisher_test.go`
- Create: `cmd/mock-function/main.go`
- Create: `cmd/mock-function/main_test.go`
- Modify: `.env.example`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Write failing config, publisher, and main tests**

Config tests must cover:

- Required HTTP address, NATS URL, stream, durable, and three app names.
- Derived command subject map:
  - `cmd.app.documents.resource.create -> documents`
  - `cmd.app.tasks.resource.create -> tasks`
  - `cmd.app.drive.resource.create -> drive`
- Derived wildcard consumer subject `cmd.app.*.resource.create`.
- Positive fetch count, max wait, publish timeout, and shutdown timeout.
- Invalid app names with dots or whitespace.

Publisher test:

```go
func TestResourceUpsertPublisherPublishesDerivedSubject(t *testing.T) {
	publisher := &fakeMessagePublisher{}
	upsertPublisher := newResourceUpsertPublisher(publisher, eventbus.WithPublishTimeout(time.Second))
	err := upsertPublisher.PublishResourceUpsert(context.Background(), mockfunction.ResourceUpsertEvent{
		ResourceID:   "resource-1",
		DisplayName:  "Docs",
		ResourceType: "document",
		ResourceTags: []string{},
		FunctionKey:  "documents",
		WorkspaceID:  "workspace-1",
		EventID:      "event-1",
		EventTime:    time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PublishResourceUpsert() error = %v", err)
	}
	if publisher.subject != "app.documents.resource.upserted" {
		t.Fatalf("subject = %q", publisher.subject)
	}
}
```

Main tests should verify `newResourceCreateEventbusConfig(cfg)` uses `Subjects: []string{"cmd.app.*.resource.create"}` and configured stream, durable, fetch count, and max wait.

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/mock-function/config ./cmd/mock-function
```

Expected: FAIL because config, publisher, and main wiring are not implemented.

- [ ] **Step 3: Implement mock-function config and publisher**

Implement config keys:

- `MOCK_FUNCTION_ENV`
- `MOCK_FUNCTION_HTTP_ADDR`
- `MOCK_FUNCTION_NATS_URL`
- `MOCK_FUNCTION_RESOURCE_CREATE_STREAM`
- `MOCK_FUNCTION_RESOURCE_CREATE_DURABLE`
- `MOCK_FUNCTION_DOCUMENTS_APP_NAME`
- `MOCK_FUNCTION_TASKS_APP_NAME`
- `MOCK_FUNCTION_DRIVE_APP_NAME`
- `MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT`
- `MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT`
- `MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT`
- `MOCK_FUNCTION_SHUTDOWN_TIMEOUT`

Implement methods:

```go
func (c Config) ResourceCreateSubjectAppNames() map[string]string
func (c Config) ResourceCreateConsumerSubject() string
```

`ResourceCreateConsumerSubject()` returns:

```go
"cmd.app.*.resource.create"
```

Implement `cmd/mock-function/resource_upsert_publisher.go` with the same shape as `cmd/function-service/resource_deleted_publisher.go`.

- [ ] **Step 4: Implement mock-function main**

`cmd/mock-function/main.go` should:

- Load config.
- Create logger.
- Connect NATS.
- Create JetStream producer.
- Create resource service with publisher, logger, clock, and UUID generator defaults.
- Create resource-create handler with `cfg.ResourceCreateSubjectAppNames()`.
- Create JetStream consumer with wildcard subject filter `cmd.app.*.resource.create`.
- Register liveness route.
- Start HTTP server and consumer as sibling goroutines.
- Close NATS connection on shutdown.

- [ ] **Step 5: Update local config and Docker Compose**

Add `.env.example` sections:

```env
# Mock HR
MOCK_HR_ENV=development
MOCK_HR_HTTP_ADDR=:8082
MOCK_HR_SHUTDOWN_TIMEOUT=10s

# Workspace service
WORKSPACE_SERVICE_ENV=development
WORKSPACE_SERVICE_HTTP_ADDR=:8083
WORKSPACE_SERVICE_MONGODB_URI=mongodb://localhost:27017
WORKSPACE_SERVICE_MONGODB_DATABASE=workspace_permission_management
WORKSPACE_SERVICE_NATS_URL=nats://localhost:4222
WORKSPACE_SERVICE_HR_BASE_URL=http://localhost:8082
WORKSPACE_SERVICE_DOCUMENTS_APP_NAME=documents
WORKSPACE_SERVICE_DOCUMENTS_RESOURCE_TYPE=document
WORKSPACE_SERVICE_TASKS_APP_NAME=tasks
WORKSPACE_SERVICE_TASKS_RESOURCE_TYPE=task
WORKSPACE_SERVICE_DRIVE_APP_NAME=drive
WORKSPACE_SERVICE_DRIVE_RESOURCE_TYPE=file
WORKSPACE_SERVICE_COMMAND_PUBLISH_TIMEOUT=15s
WORKSPACE_SERVICE_SHUTDOWN_TIMEOUT=10s

# Mock function
MOCK_FUNCTION_ENV=development
MOCK_FUNCTION_HTTP_ADDR=:8084
MOCK_FUNCTION_NATS_URL=nats://localhost:4222
MOCK_FUNCTION_RESOURCE_CREATE_STREAM=RESOURCE_CREATE_COMMANDS
MOCK_FUNCTION_RESOURCE_CREATE_DURABLE=mock-function-resource-create
MOCK_FUNCTION_DOCUMENTS_APP_NAME=documents
MOCK_FUNCTION_TASKS_APP_NAME=tasks
MOCK_FUNCTION_DRIVE_APP_NAME=drive
MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT=20
MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT=5s
MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT=15s
MOCK_FUNCTION_SHUTDOWN_TIMEOUT=10s
```

Update `docker-compose.yml`:

- Keep Go service processes outside Docker Compose because the current Compose file manages local dependencies only.
- Extend `nats-init` to create `RESOURCE_CREATE_COMMANDS` with subject `cmd.app.*.resource.create`.
- Extend `nats-init` to create durable `mock-function-resource-create` with filter `cmd.app.*.resource.create`.
- Ensure the existing function resource stream accepts `app.*.resource.upserted` for mock-function publish success in fresh local environments.

Use exact NATS stream subjects in the script:

```sh
nats --server "$$FUNCTION_SERVICE_NATS_URL" stream add "$$MOCK_FUNCTION_RESOURCE_CREATE_STREAM" \
  --subjects "cmd.app.*.resource.create" \
  --storage file \
  --retention limits \
  --discard old \
  --replicas 1 \
  --defaults
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/domain/mockfunction ./internal/mock-function/... ./cmd/mock-function
```

Expected: PASS.

Commit:

```bash
git add internal/mock-function cmd/mock-function .env.example docker-compose.yml
git commit -m "feat: wire mock function service"
```

## Task 8: End-to-End Verification and Final Plan Transition

**Files:**
- Modify: `docs/plans/active/2026-05-12-workspace-service-integration.md`

- [ ] **Step 1: Run full backend tests**

Run:

```bash
go test ./...
```

Expected: PASS for all packages.

- [ ] **Step 2: Run focused service tests**

Run:

```bash
go test ./cmd/mock-hr ./cmd/workspace-service ./cmd/mock-function
go test ./internal/mock-hr/... ./internal/workspace-service/... ./internal/mock-function/...
```

Expected: PASS.

- [ ] **Step 3: Run local smoke flow when Docker is available**

Run:

```bash
docker compose up -d mongodb mongo-init nats nats-init
```

Expected: MongoDB and NATS services are healthy, and `nats-init` completes.

Run each service locally in separate terminals:

```bash
go run ./cmd/mock-hr
go run ./cmd/mock-function
go run ./cmd/workspace-service
```

Create a workspace:

```bash
curl -sS -X POST http://localhost:8083/api/v1/workspaces \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"Delivery Workspace",
    "description":"Workspace for delivery",
    "owner":"user1",
    "documents":{"resource_name":"Delivery documents"},
    "tasks":{"resource_name":"Delivery tasks"},
    "drive":{"resource_name":"Delivery drive"}
  }'
```

Expected response includes:

```json
{
  "workspace": {
    "name": "Delivery Workspace",
    "owner": {
      "nt_account": "user1",
      "display_name": "Test User 測試員"
    }
  }
}
```

Expected logs:

- `workspace-service` logs no error for successful command publish.
- `mock-function` logs receipt for documents, tasks, and drive commands.
- `mock-function` logs successful resource upsert publish for each command.

- [ ] **Step 4: Verify REST examples are aligned**

Open and manually inspect:

```bash
sed -n '1,220p' examples/api/mock_hr.http
sed -n '1,220p' examples/api/workspaces.http
```

Expected:

- `mock_hr.http` targets mock HR's configured local port.
- `workspaces.http` targets workspace-service's configured local port.
- Request payloads match the design docs.

- [ ] **Step 5: Mark this plan completed after implementation**

After all implementation tasks pass and the code is committed, move this plan:

```bash
git mv docs/plans/active/2026-05-12-workspace-service-integration.md docs/plans/completed/2026-05-12-workspace-service-integration.md
git commit -m "docs: complete workspace service integration plan"
```

Expected: the active plan is moved to `docs/plans/completed/` in version history, matching the Design and Plan Docs Policy.

## Self-Review Checklist

- [ ] Shared HR domain and client boundary are covered by Task 1.
- [ ] Mock HR APIs, fixed display name behavior, shared response envelopes, config, health, and examples are covered by Task 2.
- [ ] Workspace request/response payloads and resource-create CloudEvent builder are covered by Task 3.
- [ ] Workspace MongoDB persistence, HR lookup, owner NT account storage, and best-effort publish workflow are covered by Task 4.
- [ ] Workspace HTTP route, error mapping, config, main wiring, publisher, and REST examples are covered by Task 5.
- [ ] Mock-function command parsing, upsert event generation, ack/retry/terminate behavior, and logging boundaries are covered by Task 6.
- [ ] Mock-function config, main wiring, publisher, `.env.example`, and Docker Compose NATS setup are covered by Task 7.
- [ ] Full verification and plan completion transition are covered by Task 8.
