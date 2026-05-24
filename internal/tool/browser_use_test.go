package tool

import (
	"testing"

	"github.com/chromedp/cdproto/cdp"
)

func testCDPNode(name string, attrs []string) *cdp.Node {
	return &cdp.Node{
		NodeName:   name,
		Attributes: attrs,
	}
}

func TestBrowserUseElementDescription(t *testing.T) {
	node := testCDPNode("INPUT", []string{"type", "email", "placeholder", "Email address"})
	got := browserUseElementDescription(node)
	want := "Input(email): Email address"
	if got != want {
		t.Fatalf("browserUseElementDescription() = %q, want %q", got, want)
	}
}

func TestBrowserUseURLChecks(t *testing.T) {
	if !browserUseAllowedURL("https://example.com") {
		t.Fatalf("https URL should be allowed")
	}
	if browserUseAllowedURL("file:///tmp/a.html") {
		t.Fatalf("file URL should not be allowed for navigation")
	}
	if !browserUseScriptableURL("about:blank") {
		t.Fatalf("about:blank should be scriptable")
	}
}
