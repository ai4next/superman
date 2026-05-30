package fs

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type LogMessage struct {
	Role    string
	Content string
	Tool    string
	Args    string
	Result  string
	Summary bool
}

func WriteSessionLogPath(path string, messages []LogMessage) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, FormatSessionLog(messages), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

func FormatSessionLog(messages []LogMessage) []byte {
	var b strings.Builder
	for _, msg := range messages {
		if msg.Summary {
			continue
		}
		switch msg.Role {
		case "user":
			writeLogLine(&b, "U", msg.Content)
		case "assistant":
			writeLogLine(&b, "A", msg.Content)
		case "tool":
			if strings.TrimSpace(msg.Args) != "" {
				writeLogLine(&b, "T", toolCallLog(msg))
			}
			if strings.TrimSpace(msg.Result) != "" {
				writeLogLine(&b, "O", toolOutputLog(msg))
			}
		case "error":
			writeLogLine(&b, "E", msg.Content)
		}
	}
	return []byte(b.String())
}

func writeLogLine(b *strings.Builder, prefix, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	quoted := strconv.Quote(value)
	fmt.Fprintf(b, "%s: %s\n", prefix, quoted[1:len(quoted)-1])
}

func toolCallLog(msg LogMessage) string {
	if strings.TrimSpace(msg.Args) == "" {
		return msg.Tool
	}
	return fmt.Sprintf("%s(%s)", msg.Tool, msg.Args)
}

func toolOutputLog(msg LogMessage) string {
	result := strings.TrimSpace(msg.Result)
	if strings.HasPrefix(result, `{"output":`) {
		if start := strings.Index(result, `:"`); start >= 0 {
			if output, err := strconv.Unquote(result[start+1 : len(result)-1]); err == nil {
				return output
			}
		}
	}
	return result
}
