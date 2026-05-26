package session

import (
	"path/filepath"
	"strings"

	adksession "google.golang.org/adk/session"
)

const ContextRecordsKey = "temp:superman.context_records"

type ContextRecords struct {
	FileReads     []string           `json:"file_reads,omitempty"`
	FileWrites    []string           `json:"file_writes,omitempty"`
	FileRevisions []FileRevisionNote `json:"file_revisions,omitempty"`
	References    []SessionReference `json:"references,omitempty"`
}

type FileRevisionNote struct {
	Path          string `json:"path"`
	Action        string `json:"action"`
	Before        string `json:"before"`
	After         string `json:"after"`
	BeforeMissing bool   `json:"before_missing,omitempty"`
	AfterMissing  bool   `json:"after_missing,omitempty"`
}

func AddFileRead(actions *adksession.EventActions, path string) {
	records := contextRecords(actions)
	records.FileReads = appendPath(records.FileReads, path)
	setContextRecords(actions, records)
}

func AddFileWrite(actions *adksession.EventActions, path string) {
	records := contextRecords(actions)
	records.FileWrites = appendPath(records.FileWrites, path)
	setContextRecords(actions, records)
}

func AddFileRevision(actions *adksession.EventActions, note FileRevisionNote) {
	note.Path = cleanContextPath(note.Path)
	if note.Path == "" {
		return
	}
	records := contextRecords(actions)
	records.FileRevisions = append(records.FileRevisions, note)
	setContextRecords(actions, records)
}

func AddSessionReference(actions *adksession.EventActions, ref SessionReference) {
	ref.SessionID = strings.TrimSpace(ref.SessionID)
	if ref.SessionID == "" {
		return
	}
	ref.Preview = compactPreview(ref.Preview, 240)
	records := contextRecords(actions)
	records.References = append(records.References, ref)
	setContextRecords(actions, records)
}

func contextRecords(actions *adksession.EventActions) ContextRecords {
	if actions == nil || actions.StateDelta == nil {
		return ContextRecords{}
	}
	switch value := actions.StateDelta[ContextRecordsKey].(type) {
	case ContextRecords:
		return value
	case *ContextRecords:
		if value != nil {
			return *value
		}
	case map[string]any:
		return contextRecordsFromMap(value)
	}
	return ContextRecords{}
}

func setContextRecords(actions *adksession.EventActions, records ContextRecords) {
	if actions == nil {
		return
	}
	if actions.StateDelta == nil {
		actions.StateDelta = make(map[string]any)
	}
	actions.StateDelta[ContextRecordsKey] = records
}

func appendPath(paths []string, path string) []string {
	path = cleanContextPath(path)
	if path == "" {
		return paths
	}
	for _, existing := range paths {
		if existing == path {
			return paths
		}
	}
	return append(paths, path)
}

func cleanContextPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return abs
}

func contextRecordsFromMap(data map[string]any) ContextRecords {
	var records ContextRecords
	records.FileReads = stringSliceFromAny(data["file_reads"])
	records.FileWrites = stringSliceFromAny(data["file_writes"])
	for _, item := range anySlice(data["file_revisions"]) {
		if note := fileRevisionNoteFromAny(item); note.Path != "" {
			records.FileRevisions = append(records.FileRevisions, note)
		}
	}
	for _, item := range anySlice(data["references"]) {
		if ref := sessionReferenceFromAny(item); strings.TrimSpace(ref.SessionID) != "" {
			records.References = append(records.References, ref)
		}
	}
	return records
}

func stringSliceFromAny(value any) []string {
	items := anySlice(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = appendPath(out, text)
		}
	}
	return out
}

func anySlice(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []string:
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out
	case []FileRevisionNote:
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out
	case []SessionReference:
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = item
		}
		return out
	default:
		return nil
	}
}

func fileRevisionNoteFromAny(value any) FileRevisionNote {
	switch note := value.(type) {
	case FileRevisionNote:
		return note
	case map[string]any:
		return FileRevisionNote{
			Path:          stringFromMap(note, "path"),
			Action:        stringFromMap(note, "action"),
			Before:        stringFromMap(note, "before"),
			After:         stringFromMap(note, "after"),
			BeforeMissing: boolFromMap(note, "before_missing"),
			AfterMissing:  boolFromMap(note, "after_missing"),
		}
	}
	return FileRevisionNote{}
}

func sessionReferenceFromAny(value any) SessionReference {
	switch ref := value.(type) {
	case SessionReference:
		return ref
	case map[string]any:
		return SessionReference{
			SessionID: stringFromMap(ref, "session_id"),
			Role:      MessageRole(stringFromMap(ref, "role")),
			Preview:   stringFromMap(ref, "preview"),
		}
	}
	return SessionReference{}
}

func stringFromMap(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

func boolFromMap(data map[string]any, key string) bool {
	value, _ := data[key].(bool)
	return value
}
