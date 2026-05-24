package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/config"
)

func TestFormatJSONResult(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{name: "empty", raw: nil, want: "null"},
		{name: "null", raw: json.RawMessage("null"), want: "null"},
		{name: "string", raw: json.RawMessage(`"hello"`), want: "hello"},
		{name: "object", raw: json.RawMessage(`{"ok":true}`), want: "{\n  \"ok\": true\n}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatJSONResult(tt.raw)
			if got != tt.want {
				t.Fatalf("formatJSONResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWrapBrowserScript(t *testing.T) {
	wrapped := wrapBrowserScript("return 42")
	if !strings.Contains(wrapped, "async ()") {
		t.Fatalf("wrapped script should be async: %s", wrapped)
	}
	if !strings.Contains(wrapped, "return 42") {
		t.Fatalf("wrapped script should contain original script: %s", wrapped)
	}
}

func TestResolveBrowserScriptReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.js")
	if err := os.WriteFile(path, []byte("return document.title"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveBrowserScript(Dependencies{Config: &config.Config{Workspace: dir}}, "script.js")
	if err != nil {
		t.Fatal(err)
	}
	if got != "return document.title" {
		t.Fatalf("resolveBrowserScript() = %q", got)
	}
}

func TestBrowserDiffSummary(t *testing.T) {
	got := browserDiffSummary("<html></html>", "<html></html>", 0)
	if got != "DOM变化量: 0 (页面无变化)" {
		t.Fatalf("browserDiffSummary() = %q", got)
	}
}

func TestGenericAgentSmartFormat(t *testing.T) {
	got := genericAgentSmartFormat("0123456789abcdefghijklmnopqrstuvwxyz", 10, " ... ")
	want := "01234 ... vwxyz"
	if got != want {
		t.Fatalf("genericAgentSmartFormat() = %q, want %q", got, want)
	}
}

func TestNormalizeTextOnlyContent(t *testing.T) {
	got := normalizeTextOnlyContent("  first\r\nsecond\rthird\n  ")
	want := "first\nsecond\nthird"
	if got != want {
		t.Fatalf("normalizeTextOnlyContent() = %q, want %q", got, want)
	}
}

func TestGenericAgentScanScriptUsesTextOnlyFlag(t *testing.T) {
	got := genericAgentScanScript(true)
	if !strings.Contains(got, "return optHTML(true);") {
		t.Fatalf("genericAgentScanScript(true) should pass true: %s", got)
	}
}
