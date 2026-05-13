package transport

import (
	"encoding/json"
	"testing"

	domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"
)

func TestNewUserResponse(t *testing.T) {
	response := NewUserResponse(domainhr.User{NTAccount: "user1", DisplayName: "Test User 測試員"})
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body["user"]["nt_account"] != "user1" || body["user"]["display_name"] != "Test User 測試員" {
		t.Fatalf("body = %#v", body)
	}
}

func TestNewUserListResponse(t *testing.T) {
	response := NewUserListResponse([]domainhr.User{
		{NTAccount: "user1", DisplayName: "Test User 測試員"},
		{NTAccount: "user2", DisplayName: "Test User 測試員"},
	})
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var body struct {
		Users []map[string]string `json:"users"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(body.Users) != 2 || body.Users[1]["nt_account"] != "user2" {
		t.Fatalf("body = %#v", body)
	}
}
