package config

import (
	"strings"
	"testing"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

func TestLoadReadsRequiredEnvironment(t *testing.T) {
	t.Setenv("GROUP_EXPIRY_SCHEDULER_ENV", "production")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_HTTP_ADDR", ":9094")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_URI", "mongodb://example:27017")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE", "wpm")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_NATS_URL", "nats://example:4222")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", "*/5 * * * * *")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS", "true")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE", "Asia/Taipei")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC+8")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE", "25")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT", "9s")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT", "15s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Production {
		t.Fatalf("Environment = %q, want production", cfg.Environment)
	}
	if cfg.HTTPAddr != ":9094" || cfg.MongoDB.URI != "mongodb://example:27017" || cfg.NATS.URL != "nats://example:4222" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.Schedule.Expression != "*/5 * * * * *" || !cfg.Schedule.WithSeconds {
		t.Fatalf("Schedule = %+v", cfg.Schedule)
	}
	if cfg.Schedule.Location.String() != "Asia/Taipei" {
		t.Fatalf("Schedule.Location = %v, want Asia/Taipei", cfg.Schedule.Location)
	}
	if cfg.BatchSize != 25 || cfg.PublishTimeout != 9*time.Second || cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("timeouts/batch = %+v", cfg)
	}
	if cfg.GroupExpiry.BucketLocation.String() != "UTC+08:00" {
		t.Fatalf("GroupExpiry.BucketLocation = %v, want UTC+08:00", cfg.GroupExpiry.BucketLocation)
	}
	if cfg.IndividualMemberExpiry.BucketLocation.String() != "UTC+08:00" {
		t.Fatalf("IndividualMemberExpiry.BucketLocation = %v, want UTC+08:00", cfg.IndividualMemberExpiry.BucketLocation)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setRequiredConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error = %v, want nil", err)
	}
	if cfg.Environment != environment.Development {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.BatchSize != 20 {
		t.Fatalf("BatchSize = %d, want 20", cfg.BatchSize)
	}
	if cfg.PublishTimeout != 15*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("timeouts = %s/%s", cfg.PublishTimeout, cfg.ShutdownTimeout)
	}
	if cfg.Schedule.WithSeconds {
		t.Fatal("Schedule.WithSeconds = true, want false")
	}
	if cfg.Schedule.Location.String() != "UTC" {
		t.Fatalf("Schedule.Location = %v, want UTC", cfg.Schedule.Location)
	}
}

func TestLoadRejectsMissingRequiredValue(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", " ")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION is required") {
		t.Fatalf("Load error = %v, want missing cron expression", err)
	}
}

func TestLoadRejectsInvalidCronExpression(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", "not cron")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION") {
		t.Fatalf("Load error = %v, want invalid cron expression", err)
	}
}

func TestLoadRejectsInvalidSchedulerTimezone(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE", "No/SuchZone")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE") {
		t.Fatalf("Load error = %v, want invalid scheduler timezone", err)
	}
}

func TestLoadRejectsInvalidBucketTimezone(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", "Asia/Taipei")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE") {
		t.Fatalf("Load error = %v, want invalid bucket timezone", err)
	}
}

func TestLoadRejectsNonPositiveValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "batch", key: "GROUP_EXPIRY_SCHEDULER_BATCH_SIZE"},
		{name: "publish timeout", key: "GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT"},
		{name: "shutdown timeout", key: "GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredConfig(t)
			t.Setenv(tt.key, "0")
			if _, err := Load(); err == nil {
				t.Fatal("Load error = nil, want error")
			}
		})
	}
}

func setRequiredConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GROUP_EXPIRY_SCHEDULER_HTTP_ADDR", ":8084")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE", "workspace_permission_management")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_NATS_URL", "nats://localhost:4222")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT", "app.todo.group.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT", "app.todo.group.individual-member.expiry.process")
	t.Setenv("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION", "* * * * *")
}
