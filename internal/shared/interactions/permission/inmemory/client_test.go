package inmemory

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	clientpermission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
)

var _ clientpermission.Client = (*Client)(nil)

func TestClientRegisterResourceAttributesReturnsNilAndLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))

	attrs := []resource.ResourceAttribute{resource.ResourceAttribute("can_edit_private_repo")}
	client := New(WithLogger(logger))

	err := client.RegisterResourceAttributes(context.Background(), "todo", attrs)
	if err != nil {
		t.Fatalf("RegisterResourceAttributes error = %v, want nil", err)
	}
	if attrs[0] != resource.ResourceAttribute("can_edit_private_repo") {
		t.Fatalf("attrs mutated to %#v", attrs)
	}

	output := logBuffer.String()
	if !strings.Contains(output, "register resource attributes with in-memory permission client") {
		t.Fatalf("log output = %q, want registration message", output)
	}
	if !strings.Contains(output, "system_id=todo") {
		t.Fatalf("log output = %q, want system_id", output)
	}
	if !strings.Contains(output, "resource_attribute_count=1") {
		t.Fatalf("log output = %q, want resource_attribute_count", output)
	}
}

func TestNewReturnsPermissionClient(t *testing.T) {
	client := New()
	if client == nil {
		t.Fatal("New() = nil, want permission client")
	}

	if err := client.RegisterResourceAttributes(context.Background(), "todo", nil); err != nil {
		t.Fatalf("RegisterResourceAttributes error = %v, want nil", err)
	}
}
