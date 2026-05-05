package eventbus

import "testing"

func TestHeadersGet(t *testing.T) {
	headers := Headers{
		"Trace-Id": {"trace-1", "trace-2"},
	}

	if got := headers.Get("Trace-Id"); got != "trace-1" {
		t.Fatalf("Get exact = %q, want %q", got, "trace-1")
	}
	if got := headers.Get("trace-id"); got != "trace-1" {
		t.Fatalf("Get case insensitive = %q, want %q", got, "trace-1")
	}
	if got := headers.Get("missing"); got != "" {
		t.Fatalf("Get missing = %q, want empty", got)
	}
}

func TestCloneHeaders(t *testing.T) {
	source := map[string][]string{
		"X-Test": {"a", "b"},
	}

	cloned := cloneHeaders(source)
	source["X-Test"][0] = "changed"

	if got := cloned.Get("X-Test"); got != "a" {
		t.Fatalf("cloned header = %q, want %q", got, "a")
	}
	if len(cloned["X-Test"]) != 2 {
		t.Fatalf("cloned values len = %d, want 2", len(cloned["X-Test"]))
	}
}
