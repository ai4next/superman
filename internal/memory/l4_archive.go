package memory

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArchiveSessions compresses old session JSONL files into summary archives.
// Returns the number of sessions archived.
func ArchiveSessions(ctx context.Context, sessionDir, memDir string, olderThan time.Duration) (int, error) {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read session dir: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	l4Dir := filepath.Join(memDir, "l4_archive")
	os.MkdirAll(l4Dir, 0755)

	var archived int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		sessionPath := filepath.Join(sessionDir, entry.Name())
		summary, err := compressSessionFile(sessionPath)
		if err != nil {
			log.Printf("[memory] L4 compress error %s: %v", entry.Name(), err)
			continue
		}

		archiveName := strings.TrimSuffix(entry.Name(), ".jsonl") + "_summary.txt"
		archivePath := filepath.Join(l4Dir, archiveName)
		if err := os.WriteFile(archivePath, []byte(summary), 0644); err != nil {
			log.Printf("[memory] L4 write error %s: %v", archiveName, err)
			continue
		}

		if err := os.Remove(sessionPath); err != nil {
			log.Printf("[memory] L4 remove error %s: %v", entry.Name(), err)
			continue
		}

		archived++
		log.Printf("[memory] L4 archived %s", entry.Name())
	}

	if archived > 0 {
		log.Printf("[memory] L4 archived %d sessions", archived)
	}
	return archived, nil
}

// compressSessionFile reads a JSONL session and returns a compressed text summary.
func compressSessionFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var (
		startTime    string
		endTime      string
		turnCount    int
		toolUseCount int
		messages     []string
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var turn int
		var ts string
		fmt.Sscanf(line, `{"turn":%d,"timestamp":"%s`, &turn, &ts)
		ts = strings.TrimSuffix(ts, `"`)
		if startTime == "" || ts < startTime {
			startTime = ts
		}
		endTime = ts
		turnCount++

		var toolCalls int
		if idx := strings.LastIndex(line, `,"tool_calls":`); idx > 0 {
			end := strings.Index(line[idx+len(`,"tool_calls":`):], `}`)
			if end > 0 {
				fmt.Sscanf(line[idx+len(`,"tool_calls":`):idx+len(`,"tool_calls":`)+end], "%d", &toolCalls)
			}
		}
		toolUseCount += toolCalls

		if parts := strings.SplitN(line, `"user_message":`, 2); len(parts) > 1 {
			if end := strings.Index(parts[1], `","agent_response"`); end > 0 {
				msg := parts[1][:end]
				msg = strings.Trim(msg, `"`)
				if len(msg) > 0 && len(messages) < 5 {
					messages = append(messages, msg)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session: %s\n", filepath.Base(path)))
	b.WriteString(fmt.Sprintf("Period: %s ~ %s\n", startTime, endTime))
	b.WriteString(fmt.Sprintf("Turns: %d\n", turnCount))
	b.WriteString(fmt.Sprintf("Tool calls: %d\n", toolUseCount))
	if len(messages) > 0 {
		b.WriteString("Topics:\n")
		for _, msg := range messages {
			truncated := msg
			if len(truncated) > 80 {
				truncated = truncated[:80] + "..."
			}
			b.WriteString("  - " + truncated + "\n")
		}
	}
	return b.String(), nil
}