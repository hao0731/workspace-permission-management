package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

type stubIndicator struct {
	name    string
	healthy bool
}

func (s stubIndicator) Name() string {
	return s.name
}

func (s stubIndicator) IsHealthy(context.Context) bool {
	return s.healthy
}

func TestHealthManagerLiveness(t *testing.T) {
	tests := []struct {
		name           string
		indicators     []Indicator
		wantStatusCode int
		wantStatus     string
		wantInfo       map[string]IndicatorStatus
		wantError      map[string]IndicatorStatus
	}{
		{
			name:           "all indicators healthy",
			indicators:     []Indicator{stubIndicator{name: "consumer", healthy: true}},
			wantStatusCode: http.StatusOK,
			wantStatus:     "ok",
			wantInfo:       map[string]IndicatorStatus{"consumer": {Status: "up"}},
			wantError:      map[string]IndicatorStatus{},
		},
		{
			name:           "one indicator unhealthy",
			indicators:     []Indicator{stubIndicator{name: "consumer", healthy: false}},
			wantStatusCode: http.StatusServiceUnavailable,
			wantStatus:     "error",
			wantInfo:       map[string]IndicatorStatus{},
			wantError:      map[string]IndicatorStatus{"consumer": {Status: "down"}},
		},
		{
			name: "mixed indicators",
			indicators: []Indicator{
				stubIndicator{name: "process", healthy: true},
				stubIndicator{name: "consumer", healthy: false},
			},
			wantStatusCode: http.StatusServiceUnavailable,
			wantStatus:     "error",
			wantInfo:       map[string]IndicatorStatus{"process": {Status: "up"}},
			wantError:      map[string]IndicatorStatus{"consumer": {Status: "down"}},
		},
		{
			name:           "empty indicators are healthy",
			indicators:     nil,
			wantStatusCode: http.StatusOK,
			wantStatus:     "ok",
			wantInfo:       map[string]IndicatorStatus{},
			wantError:      map[string]IndicatorStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			manager := NewHealthManager(tt.indicators...)
			manager.RegisterRoutes(e)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/liveness", nil)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Fatalf("status code = %d, want %d", rec.Code, tt.wantStatusCode)
			}

			var body LivenessResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Status != tt.wantStatus {
				t.Fatalf("body status = %q, want %q", body.Status, tt.wantStatus)
			}
			assertIndicatorMap(t, "info", body.Info, tt.wantInfo)
			assertIndicatorMap(t, "error", body.Error, tt.wantError)
		})
	}
}

func assertIndicatorMap(t *testing.T, name string, got, want map[string]IndicatorStatus) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s len = %d, want %d", name, len(got), len(want))
	}
	for indicator, wantStatus := range want {
		gotStatus, ok := got[indicator]
		if !ok {
			t.Fatalf("%s missing indicator %q", name, indicator)
		}
		if gotStatus != wantStatus {
			t.Fatalf("%s[%q] = %+v, want %+v", name, indicator, gotStatus, wantStatus)
		}
	}
}
