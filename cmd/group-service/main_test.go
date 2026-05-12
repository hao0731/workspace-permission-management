package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/group-service/config"
	"github.com/labstack/echo/v5"
)

func TestProcessIndicator(t *testing.T) {
	indicator := processIndicator{}
	if indicator.Name() != "process" {
		t.Fatalf("Name = %q, want process", indicator.Name())
	}
	if !indicator.IsHealthy(context.Background()) {
		t.Fatal("IsHealthy = false, want true")
	}
}

func TestRegisterHealthRoutes(t *testing.T) {
	e := echo.New()
	registerHealthRoutes(e)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/liveness", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestNewGroupExpiryEventbusConfig(t *testing.T) {
	cfg := config.Config{
		GroupExpiryCommand: config.GroupExpiryCommandConfig{
			Stream:     "GROUP_EXPIRY",
			Durable:    "group-service-expiry",
			Subject:    "app.todo.group.expiry.process",
			FetchCount: 25,
			MaxWait:    7 * time.Second,
		},
	}

	got := newGroupExpiryEventbusConfig(cfg)

	if got.Stream != "GROUP_EXPIRY" {
		t.Fatalf("Stream = %q, want GROUP_EXPIRY", got.Stream)
	}
	if got.Durable != "group-service-expiry" {
		t.Fatalf("Durable = %q, want group-service-expiry", got.Durable)
	}
	if len(got.Subjects) != 1 || got.Subjects[0] != "app.todo.group.expiry.process" {
		t.Fatalf("Subjects = %v, want [app.todo.group.expiry.process]", got.Subjects)
	}
	if got.BatchSize != 25 {
		t.Fatalf("BatchSize = %d, want 25", got.BatchSize)
	}
	if got.MaxWait != 7*time.Second {
		t.Fatalf("MaxWait = %s, want 7s", got.MaxWait)
	}
}

func TestNewIndividualMemberExpiryEventbusConfig(t *testing.T) {
	cfg := config.Config{
		IndividualMemberExpiryCommand: config.IndividualMemberExpiryCommandConfig{
			Stream:     "INDIVIDUAL_MEMBER_EXPIRY",
			Durable:    "group-service-individual-member-expiry",
			Subject:    "app.todo.group.individual-member.expiry.process",
			FetchCount: 30,
			MaxWait:    9 * time.Second,
		},
	}

	got := newIndividualMemberExpiryEventbusConfig(cfg)

	if got.Stream != "INDIVIDUAL_MEMBER_EXPIRY" {
		t.Fatalf("Stream = %q, want INDIVIDUAL_MEMBER_EXPIRY", got.Stream)
	}
	if got.Durable != "group-service-individual-member-expiry" {
		t.Fatalf("Durable = %q, want group-service-individual-member-expiry", got.Durable)
	}
	if len(got.Subjects) != 1 || got.Subjects[0] != "app.todo.group.individual-member.expiry.process" {
		t.Fatalf("Subjects = %v, want [app.todo.group.individual-member.expiry.process]", got.Subjects)
	}
	if got.BatchSize != 30 {
		t.Fatalf("BatchSize = %d, want 30", got.BatchSize)
	}
	if got.MaxWait != 9*time.Second {
		t.Fatalf("MaxWait = %s, want 9s", got.MaxWait)
	}
}
