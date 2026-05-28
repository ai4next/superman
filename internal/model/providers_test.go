package model

import "testing"

func TestModelHeaders(t *testing.T) {
	headers := modelHeaders(map[string]string{
		"X-Custom": "value",
		"":         "ignored",
	})

	if got := headers.Get("X-Custom"); got != "value" {
		t.Fatalf("X-Custom = %q, want value", got)
	}
	if _, ok := headers[""]; ok {
		t.Fatalf("empty header key should be ignored: %#v", headers)
	}
}
