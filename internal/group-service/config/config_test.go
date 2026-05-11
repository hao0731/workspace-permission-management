package config

import (
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	t.Setenv("GROUP_SERVICE_ENV", "production")
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":9090")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://example:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "wpm")
	t.Setenv("GROUP_SERVICE_NATS_URL", "nats://example:4222")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM", "GROUP_EXPIRY")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE", "group-service-expiry")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT", "25")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT", "7s")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM", "INDIVIDUAL_MEMBER_EXPIRY")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE", "group-service-individual-member-expiry")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT", "30")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT", "9s")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
	t.Setenv("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "15s")
	t.Setenv("GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS", "250")
	t.Setenv("GROUP_SERVICE_MAX_GROUPING_RULES", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Production {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Production)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.MongoDB.URI != "mongodb://example:27017" {
		t.Fatalf("MongoDB.URI = %q, want mongodb://example:27017", cfg.MongoDB.URI)
	}
	if cfg.MongoDB.Database != "wpm" {
		t.Fatalf("MongoDB.Database = %q, want wpm", cfg.MongoDB.Database)
	}
	if cfg.NATS.URL != "nats://example:4222" {
		t.Fatalf("NATS.URL = %q, want nats://example:4222", cfg.NATS.URL)
	}
	if cfg.GroupExpiryCommand.Stream != "GROUP_EXPIRY" {
		t.Fatalf("GroupExpiryCommand.Stream = %q, want GROUP_EXPIRY", cfg.GroupExpiryCommand.Stream)
	}
	if cfg.GroupExpiryCommand.Durable != "group-service-expiry" {
		t.Fatalf("GroupExpiryCommand.Durable = %q, want group-service-expiry", cfg.GroupExpiryCommand.Durable)
	}
	if cfg.GroupExpiryCommand.Subject != "app.todo.group.expiry.process" {
		t.Fatalf("GroupExpiryCommand.Subject = %q, want app.todo.group.expiry.process", cfg.GroupExpiryCommand.Subject)
	}
	if cfg.GroupExpiryCommand.FetchCount != 25 {
		t.Fatalf("GroupExpiryCommand.FetchCount = %d, want 25", cfg.GroupExpiryCommand.FetchCount)
	}
	if cfg.GroupExpiryCommand.MaxWait != 7*time.Second {
		t.Fatalf("GroupExpiryCommand.MaxWait = %s, want 7s", cfg.GroupExpiryCommand.MaxWait)
	}
	if cfg.GroupExpiryCommand.BucketTimezone != "UTC+8" {
		t.Fatalf("GroupExpiryCommand.BucketTimezone = %q, want UTC+8", cfg.GroupExpiryCommand.BucketTimezone)
	}
	if cfg.GroupExpiryCommand.BucketLocation == nil || cfg.GroupExpiryCommand.BucketLocation.String() != "UTC+08:00" {
		t.Fatalf("BucketLocation = %v, want UTC+08:00", cfg.GroupExpiryCommand.BucketLocation)
	}
	if cfg.IndividualMemberExpiryCommand.Stream != "INDIVIDUAL_MEMBER_EXPIRY" {
		t.Fatalf("IndividualMemberExpiryCommand.Stream = %q, want INDIVIDUAL_MEMBER_EXPIRY", cfg.IndividualMemberExpiryCommand.Stream)
	}
	if cfg.IndividualMemberExpiryCommand.Durable != "group-service-individual-member-expiry" {
		t.Fatalf("IndividualMemberExpiryCommand.Durable = %q, want group-service-individual-member-expiry", cfg.IndividualMemberExpiryCommand.Durable)
	}
	if cfg.IndividualMemberExpiryCommand.Subject != "app.todo.group.individual-member.expiry.process" {
		t.Fatalf("IndividualMemberExpiryCommand.Subject = %q, want app.todo.group.individual-member.expiry.process", cfg.IndividualMemberExpiryCommand.Subject)
	}
	if cfg.IndividualMemberExpiryCommand.FetchCount != 30 {
		t.Fatalf("IndividualMemberExpiryCommand.FetchCount = %d, want 30", cfg.IndividualMemberExpiryCommand.FetchCount)
	}
	if cfg.IndividualMemberExpiryCommand.MaxWait != 9*time.Second {
		t.Fatalf("IndividualMemberExpiryCommand.MaxWait = %s, want 9s", cfg.IndividualMemberExpiryCommand.MaxWait)
	}
	if cfg.IndividualMemberExpiryCommand.BucketTimezone != "UTC+8" {
		t.Fatalf("IndividualMemberExpiryCommand.BucketTimezone = %q, want UTC+8", cfg.IndividualMemberExpiryCommand.BucketTimezone)
	}
	if cfg.IndividualMemberExpiryCommand.BucketLocation == nil || cfg.IndividualMemberExpiryCommand.BucketLocation.String() != "UTC+08:00" {
		t.Fatalf("IndividualMemberExpiryCommand.BucketLocation = %v, want UTC+08:00", cfg.IndividualMemberExpiryCommand.BucketLocation)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 15s", cfg.ShutdownTimeout)
	}
	if cfg.Validation.MaxIndividualMembers != 250 {
		t.Fatalf("MaxIndividualMembers = %d, want 250", cfg.Validation.MaxIndividualMembers)
	}
	if cfg.Validation.MaxGroupingRules != 5 {
		t.Fatalf("MaxGroupingRules = %d, want 5", cfg.Validation.MaxGroupingRules)
	}
}

func TestLoadAppliesOptionalDefaults(t *testing.T) {
	setRequiredGroupServiceConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, environment.Development)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.Validation.MaxIndividualMembers != 1000 {
		t.Fatalf("MaxIndividualMembers = %d, want 1000", cfg.Validation.MaxIndividualMembers)
	}
	if cfg.Validation.MaxGroupingRules != 10 {
		t.Fatalf("MaxGroupingRules = %d, want 10", cfg.Validation.MaxGroupingRules)
	}
	if cfg.GroupExpiryCommand.FetchCount != 20 {
		t.Fatalf("GroupExpiryCommand.FetchCount = %d, want 20", cfg.GroupExpiryCommand.FetchCount)
	}
	if cfg.GroupExpiryCommand.MaxWait != 5*time.Second {
		t.Fatalf("GroupExpiryCommand.MaxWait = %s, want 5s", cfg.GroupExpiryCommand.MaxWait)
	}
	if cfg.GroupExpiryCommand.BucketTimezone != "UTC" {
		t.Fatalf("GroupExpiryCommand.BucketTimezone = %q, want UTC", cfg.GroupExpiryCommand.BucketTimezone)
	}
	if cfg.IndividualMemberExpiryCommand.FetchCount != 20 {
		t.Fatalf("IndividualMemberExpiryCommand.FetchCount = %d, want 20", cfg.IndividualMemberExpiryCommand.FetchCount)
	}
	if cfg.IndividualMemberExpiryCommand.MaxWait != 5*time.Second {
		t.Fatalf("IndividualMemberExpiryCommand.MaxWait = %s, want 5s", cfg.IndividualMemberExpiryCommand.MaxWait)
	}
	if cfg.IndividualMemberExpiryCommand.BucketTimezone != "UTC" {
		t.Fatalf("IndividualMemberExpiryCommand.BucketTimezone = %q, want UTC", cfg.IndividualMemberExpiryCommand.BucketTimezone)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_ENV", "staging")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsMissingRequiredValue(t *testing.T) {
	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsInvalidShutdownTimeout(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("Load error = nil, want error")
	}
}

func TestLoadRejectsInvalidValidationLimits(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{
			name: "max individual members",
			key:  "GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS",
		},
		{
			name: "max grouping rules",
			key:  "GROUP_SERVICE_MAX_GROUPING_RULES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredGroupServiceConfig(t)
			t.Setenv(tt.key, "0")

			if _, err := Load(); err == nil {
				t.Fatal("Load error = nil, want error")
			}
		})
	}
}

func TestLoadRejectsMissingGroupExpiryCommandConfig(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT", " ")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT is required") {
		t.Fatalf("Load() error = %v, want missing subject", err)
	}
}

func TestLoadRejectsInvalidGroupExpiryBucketTimezone(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", "Asia/Taipei")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE") {
		t.Fatalf("Load() error = %v, want invalid timezone", err)
	}
}

func TestLoadRejectsMissingIndividualMemberExpiryCommandConfig(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", " ")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT is required") {
		t.Fatalf("Load() error = %v, want missing subject", err)
	}
}

func TestLoadRejectsInvalidIndividualMemberExpiryBucketTimezone(t *testing.T) {
	setRequiredGroupServiceConfig(t)
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "Asia/Taipei")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE") {
		t.Fatalf("Load() error = %v, want invalid timezone", err)
	}
}

func setRequiredGroupServiceConfig(t *testing.T) {
	t.Helper()

	t.Setenv("GROUP_SERVICE_HTTP_ADDR", ":8081")
	t.Setenv("GROUP_SERVICE_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_SERVICE_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("GROUP_SERVICE_NATS_URL", "nats://localhost:4222")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM", "GROUP_EXPIRY")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE", "group-service-expiry")
	t.Setenv("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM", "INDIVIDUAL_MEMBER_EXPIRY")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE", "group-service-individual-member-expiry")
	t.Setenv("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
}
