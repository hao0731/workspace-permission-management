package eventbus

import "strings"

type Message struct {
	Subject string
	Data    []byte
	Headers Headers
}

type Headers map[string][]string

func (h Headers) Get(name string) string {
	if values := h[name]; len(values) > 0 {
		return values[0]
	}
	for key, values := range h {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func cloneHeaders(source map[string][]string) Headers {
	headers := make(Headers, len(source))
	for key, values := range source {
		copied := make([]string, len(values))
		copy(copied, values)
		headers[key] = copied
	}
	return headers
}
