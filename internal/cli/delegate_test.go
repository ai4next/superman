package cli

import (
	"strings"
	"testing"
)

func TestParseExpertResultJSON(t *testing.T) {
	raw := `{"success":true,"summary":"done","findings":["a"],"confidence":0.9}`
	result := parseExpertResult(raw)
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Summary != "done" {
		t.Fatalf("summary = %q", result.Summary)
	}
	if len(result.Findings) != 1 || result.Findings[0] != "a" {
		t.Fatalf("findings = %#v", result.Findings)
	}
	if result.Confidence != 0.9 {
		t.Fatalf("confidence = %f", result.Confidence)
	}
}

func TestParseExpertResultFallback(t *testing.T) {
	result := parseExpertResult("plain response")
	if !result.Success {
		t.Fatal("expected fallback success")
	}
	if !strings.Contains(result.Summary, "plain response") {
		t.Fatalf("summary = %q", result.Summary)
	}
	if result.Confidence != 0.5 {
		t.Fatalf("confidence = %f", result.Confidence)
	}
}
