package app

import (
	"strings"
	"testing"
)

func TestUnifiedDiff(t *testing.T) {
	got := unifiedDiff("main.go", "a\nold\nc\n", "a\nnew\nc\n")
	for _, want := range []string{
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,3 +1,3 @@",
		" a",
		"-old",
		"+new",
		" c",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("diff missing %q:\n%s", want, got)
		}
	}
}
