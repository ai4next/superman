package session

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	adksession "google.golang.org/adk/session"
)

type PromptReferenceCounts struct {
	Files    int `json:"files"`
	Sessions int `json:"sessions"`
}

var sessionReferencePattern = regexp.MustCompile(`\[session:([^\]\s]+)(?:\s+role:([^\]\s]+))?\]\s*([^\n]*)`)

func ExtractSessionReferences(text string) []SessionReference {
	matches := sessionReferencePattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	refs := make([]SessionReference, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		sessionID := strings.TrimSpace(match[1])
		if sessionID == "" {
			continue
		}
		role := MessageRole(strings.TrimSpace(match[2]))
		preview := strings.TrimSpace(match[3])
		key := sessionID + "\x00" + string(role) + "\x00" + preview
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, SessionReference{
			SessionID: sessionID,
			Role:      role,
			Preview:   preview,
		})
	}
	return refs
}

func ExtractFileReferences(text string) []string {
	var refs []string
	seen := make(map[string]struct{})
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '@' || i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
			continue
		}
		if i > 0 && isFileReferencePrefixChar(runes[i-1]) {
			continue
		}
		start := i + 1
		var ref string
		if runes[start] == '"' || runes[start] == '\'' {
			quote := runes[start]
			j := start + 1
			for j < len(runes) && runes[j] != quote {
				j++
			}
			if j >= len(runes) {
				continue
			}
			ref = string(runes[start+1 : j])
			i = j
		} else {
			j := start
			for j < len(runes) && !unicode.IsSpace(runes[j]) {
				j++
			}
			ref = strings.TrimRight(string(runes[start:j]), ".,;:!?)]}")
			i = j - 1
		}
		ref = strings.TrimSpace(strings.Trim(ref, "`"))
		if ref == "" || strings.Contains(ref, "://") {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

func ResolveWorkspacePath(workspace, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "~") {
		return ""
	}
	path := ref
	if !filepath.IsAbs(path) {
		if strings.TrimSpace(workspace) == "" {
			workspace = "."
		}
		path = filepath.Join(workspace, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return abs
}

func RecordPromptReferences(svc *Service, appName, userID, sessionID, workspace, prompt string) PromptReferenceCounts {
	actions := adksession.EventActions{StateDelta: make(map[string]any)}
	counts := AddPromptReferences(&actions, workspace, prompt)
	if svc == nil || counts == (PromptReferenceCounts{}) {
		return counts
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	stored, ok := svc.sessions[sessionKey(appName, userID, sessionID)]
	if !ok {
		return PromptReferenceCounts{}
	}
	if err := svc.applyContextRecordsLocked(stored, actions.StateDelta); err != nil {
		return PromptReferenceCounts{}
	}
	stored.UpdatedAt = time.Now()
	if err := svc.persistLocked(stored); err != nil {
		return PromptReferenceCounts{}
	}
	return counts
}

func AddPromptReferences(actions *adksession.EventActions, workspace, prompt string) PromptReferenceCounts {
	var counts PromptReferenceCounts
	for _, ref := range ExtractFileReferences(prompt) {
		path := ResolveWorkspacePath(workspace, ref)
		if path == "" {
			continue
		}
		AddFileRead(actions, path)
		counts.Files++
	}
	for _, ref := range ExtractSessionReferences(prompt) {
		AddSessionReference(actions, ref)
		counts.Sessions++
	}
	return counts
}

func isFileReferencePrefixChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '/' || r == ':' || r == '_' || r == '-' || r == '.'
}
