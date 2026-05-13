package transport

import (
	"strings"
	"testing"
)

func TestDecodeUserListRequest(t *testing.T) {
	request, err := DecodeUserListRequest(strings.NewReader(`{"nt_accounts":[" user1 ","user2"]}`))
	if err != nil {
		t.Fatalf("DecodeUserListRequest() error = %v", err)
	}
	accounts, err := request.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain() error = %v", err)
	}
	if len(accounts) != 2 || accounts[0] != "user1" || accounts[1] != "user2" {
		t.Fatalf("accounts = %#v", accounts)
	}
}

func TestDecodeUserListRequestRejectsEmptyList(t *testing.T) {
	_, err := DecodeUserListRequest(strings.NewReader(`{"nt_accounts":[]}`))
	if err == nil {
		t.Fatal("DecodeUserListRequest() error = nil, want error")
	}
}

func TestDecodeUserListRequestRejectsMissingList(t *testing.T) {
	_, err := DecodeUserListRequest(strings.NewReader(`{}`))
	if err == nil {
		t.Fatal("DecodeUserListRequest() error = nil, want error")
	}
}

func TestDecodeUserListRequestRejectsEmptyAccount(t *testing.T) {
	request, err := DecodeUserListRequest(strings.NewReader(`{"nt_accounts":["user1"," "]}`))
	if err != nil {
		t.Fatalf("DecodeUserListRequest() error = %v", err)
	}
	if _, err := request.ToDomain(); err == nil {
		t.Fatal("ToDomain() error = nil, want error")
	}
}

func TestDecodeUserListRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeUserListRequest(strings.NewReader(`{"nt_accounts":`))
	if err == nil {
		t.Fatal("DecodeUserListRequest() error = nil, want error")
	}
}
