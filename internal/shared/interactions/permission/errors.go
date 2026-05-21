package permission

import "fmt"

type ErrorResponse struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type Error struct {
	StatusCode int
	Response   ErrorResponse
}

func (e *Error) Error() string {
	if e.Response.Message != "" {
		return fmt.Sprintf("permission API request failed with status %d: %s", e.StatusCode, e.Response.Message)
	}
	return fmt.Sprintf("permission API request failed with status %d", e.StatusCode)
}
