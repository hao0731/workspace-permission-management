package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestWriteRelationshipsLogsPayloadAndReturnsResult(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

	e := echo.New()
	RegisterRoutes(e, NewSchemaHandler(logger))

	body := `{
		"updates": [
			{
				"operation": "create",
				"relationship": {
					"relation": "hr_member",
					"resource": {"object_id": "group-1", "object_type": "group"},
					"subject": {
						"object": {"object_id": "ORG-100", "object_type": "organization"},
						"optionalRelation": "member"
					}
				}
			}
		]
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/relationships/write", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Writes []struct {
			Success bool `json:"success"`
		} `json:"writes"`
		Deletes []struct {
			Success bool `json:"success"`
		} `json:"deletes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Writes) != 1 || !response.Writes[0].Success {
		t.Fatalf("writes = %+v, want one success", response.Writes)
	}
	output := logBuffer.String()
	if !strings.Contains(output, "mock permission relationships write received") {
		t.Fatalf("log output = %q, want relationships write message", output)
	}
	if !strings.Contains(output, "update_count=1") {
		t.Fatalf("log output = %q, want update_count", output)
	}
}

func TestWriteRelationshipsRejectsMalformedJSON(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, NewSchemaHandler(slog.Default()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/relationships/write", strings.NewReader(`{`))
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
