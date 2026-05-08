package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func EncodeNextToken[T any](input T) (string, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal next token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeNextToken[T any](raw string) (T, error) {
	var out T
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return out, fmt.Errorf("next_token must be base64url encoded JSON")
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("next_token must be JSON")
	}
	return out, nil
}
