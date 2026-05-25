package session

import (
	"strings"
	"testing"
)

func TestUnifiedDiff(t *testing.T) {
	got := UnifiedDiff("file.txt", "old\nsame\n", "new\nsame\n")
	for _, want := range []string{
		"--- a/file.txt",
		"+++ b/file.txt",
		"-old",
		"+new",
		" same",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("diff missing %q:\n%s", want, got)
		}
	}
}
