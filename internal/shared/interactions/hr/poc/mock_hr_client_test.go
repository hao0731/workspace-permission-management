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
