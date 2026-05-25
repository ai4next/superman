package app

import (
	"fmt"
	"strings"

	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tui/components"
)

func formatFileRevisionDiff(rev supermansession.FileRevision) string {
	var b strings.Builder
	b.WriteString("Latest file revision\n")
	b.WriteString("Path: ")
	b.WriteString(rev.Path)
	b.WriteString("\nAction: ")
	b.WriteString(rev.Action)
	b.WriteString(fmt.Sprintf("\nSize: %d -> %d bytes", rev.Before.Size, rev.After.Size))
	if rev.Before.Missing {
		b.WriteString("\nBefore: <missing>")
		b.WriteString("\n\nUnified diff:\n")
		b.WriteString(components.TruncateRunes(unifiedDiff(rev.Path, "", rev.After.Preview), 3200))
		return b.String()
	}
	b.WriteString("\n\nUnified diff:\n")
	b.WriteString(components.TruncateRunes(unifiedDiff(rev.Path, rev.Before.Preview, rev.After.Preview), 3200))
	return b.String()
}

func unifiedDiff(path, before, after string) string {
	return supermansession.UnifiedDiff(path, before, after)
}
