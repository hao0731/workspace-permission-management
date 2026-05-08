package exception

import "testing"

func TestNew_BuildsExceptionWithoutDetails(t *testing.T) {
	ex := New("validation_failed", "invalid request")
	if ex.Code != "validation_failed" || ex.Message != "invalid request" {
		t.Fatalf("unexpected exception: %+v", ex)
	}
	if ex.Details != nil {
		t.Fatalf("expected nil details, got %+v", ex.Details)
	}
}

func TestNew_WithDetails(t *testing.T) {
	details := map[string]any{"field": "workspace_id"}
	ex := New("validation_failed", "invalid request", WithDetails(details))
	if ex.Details["field"] != "workspace_id" {
		t.Fatalf("unexpected details: %+v", ex.Details)
	}
}

func TestNew_WithRequestId(t *testing.T) {
	ex := New("validation_failed", "invalid request", WithRequestId("req-123"))
	if ex.RequestID != "req-123" {
		t.Fatalf("unexpected request id: %+v", ex)
	}
}

func TestWrapResponse(t *testing.T) {
	ex := New("internal_error", "failed")
	resp := WrapResponse(ex)
	if resp.Error.Code != "internal_error" {
		t.Fatalf("unexpected wrapped response: %+v", resp)
	}
}
