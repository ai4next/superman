package session

import (
	"fmt"
	"strings"
)

func UnifiedDiff(path, before, after string) string {
	if before == after {
		return fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path)
	}
	beforeLines := splitDiffLines(before)
	afterLines := splitDiffLines(after)
	ops := diffOps(beforeLines, afterLines)

	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", path, path)
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", max(1, len(beforeLines)), max(1, len(afterLines)))
	for _, op := range ops {
		switch op.kind {
		case diffEqual:
			b.WriteString(" ")
		case diffDelete:
			b.WriteString("-")
		case diffInsert:
			b.WriteString("+")
		}
		b.WriteString(op.text)
		if !strings.HasSuffix(op.text, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

type diffKind int

const (
	diffEqual diffKind = iota
	diffDelete
	diffInsert
)

type diffOp struct {
	kind diffKind
	text string
}

func diffOps(a, b []string) []diffOp {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var ops []diffOp
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			ops = append(ops, diffOp{kind: diffEqual, text: a[i]})
			i++
			j++
		} else if lcs[i+1][j] >= lcs[i][j+1] {
			ops = append(ops, diffOp{kind: diffDelete, text: a[i]})
			i++
		} else {
			ops = append(ops, diffOp{kind: diffInsert, text: b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, diffOp{kind: diffDelete, text: a[i]})
	}
	for ; j < len(b); j++ {
		ops = append(ops, diffOp{kind: diffInsert, text: b[j]})
	}
	return ops
}

func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
