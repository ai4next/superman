package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	supermansession "github.com/ai4next/superman/internal/session"
	adksession "google.golang.org/adk/session"
)

var (
	sessionsUserID  string
	sessionsJSON    bool
	queuePrompt     string
	queueFile       string
	sessionExportAs string
	compactMax      int
	compactKeep     int
	compactSummary  int
	importOverwrite bool
	searchRole      string
	searchLimit     int
	storageGCApply  bool
)

const timeFormat = "2006-01-02 15:04:05"

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Inspect persistent Superman sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List persistent sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		return writeSessionList(os.Stdout, svc, cfg, sessionsUserID, sessionsJSON)
	},
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show session messages",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionShow(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionsJSON)
	},
}

var sessionsLastCmd = &cobra.Command{
	Use:   "last",
	Short: "Show the most recently updated session",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		return writeSessionLast(os.Stdout, svc, cfg, sessionsUserID, sessionsJSON)
	},
}

var sessionsSearchCmd = &cobra.Command{
	Use:   "search <query> [session-id]",
	Short: "Search persisted session messages",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID := ""
		if len(args) > 1 {
			sessionID, err = resolveSessionID(svc, cfg, sessionsUserID, args[1])
			if err != nil {
				return err
			}
		}
		return writeSessionSearch(os.Stdout, svc, cfg, sessionsUserID, args[0], sessionID, searchRole, searchLimit, sessionsJSON)
	},
}

var sessionsFilesCmd = &cobra.Command{
	Use:   "files <session-id>",
	Short: "Show session working files",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionFiles(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionsJSON)
	},
}

var sessionsHistoryCmd = &cobra.Command{
	Use:   "history <session-id>",
	Short: "Show session file revision history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionHistory(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionsJSON)
	},
}

var sessionsDiffCmd = &cobra.Command{
	Use:   "diff <session-id> <path>",
	Short: "Show the latest session revision diff for a file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionDiff(os.Stdout, svc, cfg, sessionsUserID, sessionID, args[1], sessionsJSON)
	},
}

var sessionsRevertCmd = &cobra.Command{
	Use:   "revert <session-id> <path>",
	Short: "Revert a file to its previous session revision",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionRevert(os.Stdout, svc, cfg, sessionsUserID, sessionID, args[1], sessionsJSON)
	},
}

var sessionsExportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export a session transcript and file history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionExport(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionExportAs)
	},
}

var sessionsImportCmd = &cobra.Command{
	Use:   "import <path>",
	Short: "Import a JSON or JSONL session export",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		return writeSessionImport(os.Stdout, svc, cfg, sessionsUserID, args[0], importOverwrite, sessionsJSON)
	},
}

var sessionsCompactCmd = &cobra.Command{
	Use:   "compact <session-id>",
	Short: "Compact older session context into a summary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionCompact(os.Stdout, svc, cfg, sessionsUserID, sessionID, compactOptions(cfg), sessionsJSON)
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a persistent session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionDelete(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionsJSON)
	},
}

var sessionsRenameCmd = &cobra.Command{
	Use:   "rename <session-id> <title>",
	Short: "Rename a persistent session",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionRename(os.Stdout, svc, cfg, sessionsUserID, sessionID, args[1], sessionsJSON)
	},
}

var sessionsQueueCmd = &cobra.Command{
	Use:   "queue <session-id>",
	Short: "Inspect or manage queued prompts for a session",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionQueue(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionsJSON)
	},
}

var sessionsQueueListCmd = &cobra.Command{
	Use:   "list <session-id>",
	Short: "List queued prompts for a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionQueue(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionsJSON)
	},
}

var sessionsQueueAddCmd = &cobra.Command{
	Use:   "add <session-id> [prompt]",
	Short: "Append a prompt to a session queue",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		prompt, err := queuePromptInput(args)
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionQueueAdd(os.Stdout, svc, cfg, sessionsUserID, sessionID, prompt, sessionsJSON)
	},
}

var sessionsQueueClearCmd = &cobra.Command{
	Use:   "clear <session-id>",
	Short: "Clear queued prompts for a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		sessionID, err := resolveSessionID(svc, cfg, sessionsUserID, args[0])
		if err != nil {
			return err
		}
		return writeSessionQueueClear(os.Stdout, svc, cfg, sessionsUserID, sessionID, sessionsJSON)
	},
}

var sessionsStorageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Inspect persistent session storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		return writeSessionStorage(os.Stdout, svc, cfg, sessionsJSON)
	},
}

var sessionsStorageGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Remove orphaned file revision snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, err := openSessionService()
		if err != nil {
			return err
		}
		return writeSessionStorageGC(os.Stdout, svc, cfg, !storageGCApply, sessionsJSON)
	},
}

func init() {
	sessionsCmd.PersistentFlags().StringVar(&sessionsUserID, "user", "tui-user", "session user id")
	sessionsCmd.PersistentFlags().BoolVar(&sessionsJSON, "json", false, "print JSON")
	sessionsQueueAddCmd.Flags().StringVarP(&queuePrompt, "prompt", "p", "", "prompt text")
	sessionsQueueAddCmd.Flags().StringVarP(&queueFile, "file", "f", "", "read prompt from file")
	sessionsExportCmd.Flags().StringVar(&sessionExportAs, "format", "markdown", "export format: markdown, json, or jsonl")
	sessionsImportCmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "overwrite an existing session with the same id")
	sessionsCompactCmd.Flags().IntVar(&compactMax, "max-messages", 0, "max non-summary messages before compaction")
	sessionsCompactCmd.Flags().IntVar(&compactKeep, "keep-last", 0, "recent non-summary messages to keep outside summary")
	sessionsCompactCmd.Flags().IntVar(&compactSummary, "max-summary-runes", 0, "max runes in generated summary")
	sessionsSearchCmd.Flags().StringVar(&searchRole, "role", "", "filter by message role: user, assistant, tool, or error")
	sessionsSearchCmd.Flags().IntVar(&searchLimit, "limit", 20, "max search results")
	sessionsStorageGCCmd.Flags().BoolVar(&storageGCApply, "apply", false, "delete orphaned snapshots instead of dry-run")
	sessionsQueueCmd.AddCommand(sessionsQueueListCmd, sessionsQueueAddCmd, sessionsQueueClearCmd)
	sessionsStorageCmd.AddCommand(sessionsStorageGCCmd)
	sessionsCmd.AddCommand(sessionsListCmd, sessionsShowCmd, sessionsLastCmd, sessionsSearchCmd, sessionsFilesCmd, sessionsHistoryCmd, sessionsDiffCmd, sessionsRevertCmd, sessionsExportCmd, sessionsImportCmd, sessionsCompactCmd, sessionsDeleteCmd, sessionsRenameCmd, sessionsQueueCmd, sessionsStorageCmd)
}

func openSessionService() (adksession.Service, *config.Config, error) {
	cfg := global.Config()
	svc, err := supermansession.NewService()
	if err != nil {
		return nil, nil, fmt.Errorf("create session service: %w", err)
	}
	return svc, cfg, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeSessionList(w io.Writer, svc adksession.Service, cfg *config.Config, userID string, asJSON bool) error {
	sessions := supermansession.ListSessionMetadata(svc, cfg.Session.AppName, userID)
	if asJSON {
		return writeJSON(w, sessions)
	}
	if len(sessions) == 0 {
		_, err := fmt.Fprintf(w, "No sessions for app=%s user=%s\n", cfg.Session.AppName, userID)
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tTITLE\tMESSAGES\tFILES\tQUEUE\tUPDATED")
	for _, meta := range sessions {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%s\n",
			meta.SessionID,
			meta.Title,
			meta.MessageCount,
			meta.FileCount,
			meta.QueuedPrompts,
			meta.UpdatedAt.Format(timeFormat),
		)
	}
	return tw.Flush()
}

func writeSessionShow(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID string, asJSON bool) error {
	messages, err := supermansession.Messages(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, messages)
	}
	if len(messages) == 0 {
		_, err := fmt.Fprintln(w, "No messages")
		return err
	}
	for _, msg := range messages {
		fmt.Fprintf(w, "[%s]", msg.Role)
		if msg.ToolName != "" {
			fmt.Fprintf(w, " %s", msg.ToolName)
		}
		if msg.Status != "" {
			fmt.Fprintf(w, " (%s)", msg.Status)
		}
		fmt.Fprintln(w)
		text := firstNonEmpty(msg.Content, msg.Result, msg.Args)
		if text != "" {
			fmt.Fprintln(w, indentLines(strings.TrimSpace(text), "  "))
		}
	}
	return nil
}

func writeSessionLast(w io.Writer, svc adksession.Service, cfg *config.Config, userID string, asJSON bool) error {
	sessions := supermansession.ListSessionMetadata(svc, cfg.Session.AppName, userID)
	if len(sessions) == 0 {
		return fmt.Errorf("no sessions for app=%s user=%s", cfg.Session.AppName, userID)
	}
	return writeSessionShow(w, svc, cfg, userID, sessions[0].SessionID, asJSON)
}

func writeSessionSearch(w io.Writer, svc adksession.Service, cfg *config.Config, userID, query, sessionID, role string, limit int, asJSON bool) error {
	roles, err := parseMessageRoles(role)
	if err != nil {
		return err
	}
	results, err := supermansession.SearchMessages(svc, cfg.Session.AppName, userID, supermansession.MessageSearchOptions{
		Query:     query,
		SessionID: sessionID,
		Roles:     roles,
		Limit:     limit,
	})
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, results)
	}
	if len(results) == 0 {
		_, err := fmt.Fprintln(w, "No matching messages")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tTITLE\tROLE\tTIME\tPREVIEW")
	for _, result := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			result.Metadata.SessionID,
			result.Metadata.Title,
			result.Message.Role,
			result.Message.CreatedAt.Format(timeFormat),
			singleLine(result.Preview),
		)
	}
	return tw.Flush()
}

func writeSessionFiles(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID string, asJSON bool) error {
	files, err := supermansession.SessionFiles(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, files)
	}
	if len(files) == 0 {
		_, err := fmt.Fprintln(w, "No files recorded")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PATH\tREADS\tWRITES\tLAST")
	for _, file := range files {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\n", file.Path, file.ReadCount, file.WriteCount, file.LastAccess)
	}
	return tw.Flush()
}

func writeSessionHistory(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID string, asJSON bool) error {
	revisions, err := supermansession.FileRevisions(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, revisions)
	}
	if len(revisions) == 0 {
		_, err := fmt.Fprintln(w, "No file history recorded")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tACTION\tPATH\tBEFORE\tAFTER")
	for _, revision := range revisions {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\n",
			revision.CreatedAt.Format(timeFormat),
			revision.Action,
			revision.Path,
			revision.Before.Size,
			revision.After.Size,
		)
	}
	return tw.Flush()
}

func writeSessionDiff(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID, path string, asJSON bool) error {
	revision, ok, err := latestSessionFileRevision(svc, cfg, userID, sessionID, path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no file history found for %s", path)
	}
	before, beforeMissing, err := supermansession.FileSnapshotContent(revision.Before)
	if err != nil {
		return err
	}
	after, afterMissing, err := supermansession.FileSnapshotContent(revision.After)
	if err != nil {
		return err
	}
	diff := supermansession.UnifiedDiff(revision.Path, before, after)
	if asJSON {
		return writeJSON(w, map[string]any{
			"session_id":     sessionID,
			"revision":       revision,
			"before_missing": beforeMissing,
			"after_missing":  afterMissing,
			"diff":           diff,
		})
	}
	_, err = fmt.Fprint(w, diff)
	return err
}

func latestSessionFileRevision(svc adksession.Service, cfg *config.Config, userID, sessionID, path string) (supermansession.FileRevision, bool, error) {
	target, err := filepath.Abs(path)
	if err != nil {
		return supermansession.FileRevision{}, false, fmt.Errorf("invalid path: %w", err)
	}
	revisions, err := supermansession.FileRevisions(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return supermansession.FileRevision{}, false, err
	}
	for i := len(revisions) - 1; i >= 0; i-- {
		if revisions[i].Path == target {
			return revisions[i], true, nil
		}
	}
	return supermansession.FileRevision{}, false, nil
}

func writeSessionRevert(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID, path string, asJSON bool) error {
	revision, ok, err := latestSessionFileRevision(svc, cfg, userID, sessionID, path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no file history found for %s", path)
	}

	current, currentMissing, err := readSessionFileSnapshot(revision.Path)
	if err != nil {
		return err
	}
	before, beforeMissing, err := supermansession.FileSnapshotContent(revision.Before)
	if err != nil {
		return err
	}

	var revertRevision supermansession.FileRevision
	if beforeMissing {
		if err := os.Remove(revision.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		revertRevision, err = supermansession.RecordFileRevisionWithMissing(svc, cfg.Session.AppName, userID, sessionID, revision.Path, "revert", current, "", currentMissing, true)
	} else {
		if err := os.MkdirAll(filepath.Dir(revision.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(revision.Path, []byte(before), 0o644); err != nil {
			return err
		}
		revertRevision, err = supermansession.RecordFileRevisionWithMissing(svc, cfg.Session.AppName, userID, sessionID, revision.Path, "revert", current, before, currentMissing, false)
	}
	if err != nil {
		return err
	}

	if asJSON {
		return writeJSON(w, map[string]any{
			"session_id": sessionID,
			"path":       revision.Path,
			"reverted":   true,
			"revision":   revertRevision,
		})
	}
	_, err = fmt.Fprintf(w, "Reverted %s for session %s\n", revision.Path, sessionID)
	return err
}

func readSessionFileSnapshot(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), false, nil
	}
	if os.IsNotExist(err) {
		return "", true, nil
	}
	return "", false, err
}

type sessionExport struct {
	Metadata      supermansession.Metadata            `json:"metadata"`
	Messages      []supermansession.Message           `json:"messages"`
	Files         []supermansession.SessionFile       `json:"files,omitempty"`
	FileRevisions []supermansession.FileRevision      `json:"file_revisions,omitempty"`
	FileChanges   []supermansession.FileChangeSummary `json:"file_changes,omitempty"`
	PromptQueue   []supermansession.QueuedPrompt      `json:"prompt_queue,omitempty"`
	References    []supermansession.SessionReference  `json:"references,omitempty"`
}

func writeSessionExport(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID, format string) error {
	export, err := buildSessionExport(svc, cfg, userID, sessionID)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "markdown", "md":
		return writeSessionExportMarkdown(w, export)
	case "json":
		return writeJSON(w, export)
	case "jsonl":
		return writeSessionExportJSONL(w, export)
	default:
		return fmt.Errorf("unsupported export format %q", format)
	}
}

func buildSessionExport(svc adksession.Service, cfg *config.Config, userID, sessionID string) (sessionExport, error) {
	meta, err := supermansession.SessionMetadata(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return sessionExport{}, err
	}
	messages, err := supermansession.Messages(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return sessionExport{}, err
	}
	files, err := supermansession.SessionFiles(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return sessionExport{}, err
	}
	revisions, err := supermansession.FileRevisions(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return sessionExport{}, err
	}
	changes, err := supermansession.SessionFileChanges(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return sessionExport{}, err
	}
	queue, err := supermansession.PromptQueue(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return sessionExport{}, err
	}
	references, err := supermansession.SessionReferences(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return sessionExport{}, err
	}
	return sessionExport{
		Metadata:      meta,
		Messages:      messages,
		Files:         files,
		FileRevisions: revisions,
		FileChanges:   changes,
		PromptQueue:   queue,
		References:    references,
	}, nil
}

func writeSessionExportMarkdown(w io.Writer, export sessionExport) error {
	fmt.Fprintf(w, "# Superman Session Export\n\n")
	fmt.Fprintf(w, "- Session: %s\n", export.Metadata.SessionID)
	fmt.Fprintf(w, "- Title: %s\n", export.Metadata.Title)
	fmt.Fprintf(w, "- App: %s\n", export.Metadata.AppName)
	fmt.Fprintf(w, "- User: %s\n", export.Metadata.UserID)
	fmt.Fprintf(w, "- Messages: %d\n", export.Metadata.MessageCount)
	fmt.Fprintf(w, "- Files: %d\n", export.Metadata.FileCount)
	fmt.Fprintf(w, "- Updated: %s\n\n", export.Metadata.UpdatedAt.Format(timeFormat))
	if len(export.Files) > 0 {
		fmt.Fprintln(w, "## Working Files")
		fmt.Fprintln(w)
		for _, file := range export.Files {
			fmt.Fprintf(w, "- `%s`", file.Path)
			var parts []string
			if file.ReadCount > 0 {
				parts = append(parts, fmt.Sprintf("reads=%d", file.ReadCount))
			}
			if file.WriteCount > 0 {
				parts = append(parts, fmt.Sprintf("writes=%d", file.WriteCount))
			}
			if len(parts) > 0 {
				fmt.Fprintf(w, " (%s)", strings.Join(parts, ", "))
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}
	if len(export.FileChanges) > 0 {
		fmt.Fprintln(w, "## File Changes")
		fmt.Fprintln(w)
		for _, change := range export.FileChanges {
			fmt.Fprintf(w, "- `%s`: +%d -%d\n", change.File.Path, change.Additions, change.Deletions)
		}
		fmt.Fprintln(w)
	}
	if len(export.References) > 0 {
		fmt.Fprintln(w, "## Session References")
		fmt.Fprintln(w)
		for _, ref := range export.References {
			fmt.Fprintf(w, "- `%s` [%s]: %s\n", ref.SessionID, ref.Role, singleLine(ref.Preview))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "## Messages")
	fmt.Fprintln(w)
	for _, msg := range export.Messages {
		if msg.Summary {
			continue
		}
		fmt.Fprintf(w, "### %s", msg.Role)
		if msg.ToolName != "" {
			fmt.Fprintf(w, " / %s", msg.ToolName)
		}
		if msg.Status != "" {
			fmt.Fprintf(w, " [%s]", msg.Status)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w)
		if msg.Content != "" {
			fmt.Fprintln(w, strings.TrimSpace(msg.Content))
			fmt.Fprintln(w)
		}
		if msg.Args != "" {
			fmt.Fprintln(w, "Args:")
			fmt.Fprintln(w, "```json")
			fmt.Fprintln(w, msg.Args)
			fmt.Fprintln(w, "```")
			fmt.Fprintln(w)
		}
		if msg.Result != "" {
			fmt.Fprintln(w, "Result:")
			fmt.Fprintln(w, "```json")
			fmt.Fprintln(w, msg.Result)
			fmt.Fprintln(w, "```")
			fmt.Fprintln(w)
		}
	}
	return nil
}

func writeSessionExportJSONL(w io.Writer, export sessionExport) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]any{"type": "metadata", "data": export.Metadata}); err != nil {
		return err
	}
	for _, msg := range export.Messages {
		if err := enc.Encode(map[string]any{"type": "message", "data": msg}); err != nil {
			return err
		}
	}
	for _, file := range export.Files {
		if err := enc.Encode(map[string]any{"type": "file", "data": file}); err != nil {
			return err
		}
	}
	for _, revision := range export.FileRevisions {
		if err := enc.Encode(map[string]any{"type": "file_revision", "data": revision}); err != nil {
			return err
		}
	}
	for _, queued := range export.PromptQueue {
		if err := enc.Encode(map[string]any{"type": "queued_prompt", "data": queued}); err != nil {
			return err
		}
	}
	for _, ref := range export.References {
		if err := enc.Encode(map[string]any{"type": "session_reference", "data": ref}); err != nil {
			return err
		}
	}
	return nil
}

func writeSessionImport(w io.Writer, svc adksession.Service, cfg *config.Config, userID, path string, overwrite bool, asJSON bool) error {
	export, err := readSessionImport(path)
	if err != nil {
		return err
	}
	meta, err := supermansession.Import(svc, cfg.Session.AppName, userID, supermansession.ImportData{
		Metadata:      export.Metadata,
		Messages:      export.Messages,
		Files:         export.Files,
		FileRevisions: export.FileRevisions,
		PromptQueue:   export.PromptQueue,
		References:    export.References,
		Overwrite:     overwrite,
	})
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, meta)
	}
	_, err = fmt.Fprintf(w, "Imported session %s (%d messages, %d files)\n", meta.SessionID, meta.MessageCount, meta.FileCount)
	return err
}

func readSessionImport(path string) (sessionExport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionExport{}, fmt.Errorf("read import: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return sessionExport{}, fmt.Errorf("import file is empty")
	}
	var export sessionExport
	if err := json.Unmarshal(data, &export); err == nil {
		return export, nil
	}
	return readSessionImportJSONL(strings.NewReader(trimmed))
}

func readSessionImportJSONL(r io.Reader) (sessionExport, error) {
	var export sessionExport
	scanner := bufio.NewScanner(r)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var item struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(text), &item); err != nil {
			return sessionExport{}, fmt.Errorf("decode import jsonl line %d: %w", line, err)
		}
		switch item.Type {
		case "metadata":
			if err := json.Unmarshal(item.Data, &export.Metadata); err != nil {
				return sessionExport{}, fmt.Errorf("decode metadata line %d: %w", line, err)
			}
		case "message":
			var msg supermansession.Message
			if err := json.Unmarshal(item.Data, &msg); err != nil {
				return sessionExport{}, fmt.Errorf("decode message line %d: %w", line, err)
			}
			export.Messages = append(export.Messages, msg)
		case "file":
			var file supermansession.SessionFile
			if err := json.Unmarshal(item.Data, &file); err != nil {
				return sessionExport{}, fmt.Errorf("decode file line %d: %w", line, err)
			}
			export.Files = append(export.Files, file)
		case "file_revision":
			var revision supermansession.FileRevision
			if err := json.Unmarshal(item.Data, &revision); err != nil {
				return sessionExport{}, fmt.Errorf("decode file_revision line %d: %w", line, err)
			}
			export.FileRevisions = append(export.FileRevisions, revision)
		case "queued_prompt":
			var queued supermansession.QueuedPrompt
			if err := json.Unmarshal(item.Data, &queued); err != nil {
				return sessionExport{}, fmt.Errorf("decode queued_prompt line %d: %w", line, err)
			}
			export.PromptQueue = append(export.PromptQueue, queued)
		case "session_reference":
			var ref supermansession.SessionReference
			if err := json.Unmarshal(item.Data, &ref); err != nil {
				return sessionExport{}, fmt.Errorf("decode session_reference line %d: %w", line, err)
			}
			export.References = append(export.References, ref)
		default:
			return sessionExport{}, fmt.Errorf("unsupported import jsonl type %q on line %d", item.Type, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return sessionExport{}, fmt.Errorf("scan import jsonl: %w", err)
	}
	if export.Metadata.SessionID == "" {
		return sessionExport{}, fmt.Errorf("import missing metadata session_id")
	}
	return export, nil
}

func compactOptions(cfg *config.Config) supermansession.CompactOptions {
	opts := supermansession.CompactOptions{
		MaxMessages: cfg.Session.MaxTurns,
		KeepLast:    20,
	}
	if compactMax > 0 {
		opts.MaxMessages = compactMax
	}
	if compactKeep > 0 {
		opts.KeepLast = compactKeep
	}
	if compactSummary > 0 {
		opts.MaxSummaryRunes = compactSummary
	}
	return opts
}

func writeSessionCompact(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID string, opts supermansession.CompactOptions, asJSON bool) error {
	result, err := supermansession.Compact(svc, cfg.Session.AppName, userID, sessionID, opts)
	if err != nil {
		return err
	}
	output := map[string]any{
		"session_id": sessionID,
		"compacted":  result.Compacted,
		"scanned":    result.Scanned,
		"kept":       result.Kept,
	}
	if result.Summary.ID != "" {
		output["summary_message_id"] = result.Summary.ID
	}
	if asJSON {
		return writeJSON(w, output)
	}
	if result.Compacted {
		_, err = fmt.Fprintf(w, "Compacted session %s: scanned=%d kept=%d summary=%s\n", sessionID, result.Scanned, result.Kept, result.Summary.ID)
		return err
	}
	_, err = fmt.Fprintf(w, "Session %s did not need compaction: scanned=%d kept=%d\n", sessionID, result.Scanned, result.Kept)
	return err
}

func writeSessionDelete(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID string, asJSON bool) error {
	if err := svc.Delete(context.Background(), &adksession.DeleteRequest{
		AppName:   cfg.Session.AppName,
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, map[string]any{
			"session_id": sessionID,
			"deleted":    true,
		})
	}
	_, err := fmt.Fprintf(w, "Deleted session %s\n", sessionID)
	return err
}

func writeSessionRename(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID, title string, asJSON bool) error {
	if err := supermansession.Rename(svc, cfg.Session.AppName, userID, sessionID, title); err != nil {
		return err
	}
	meta, err := supermansession.SessionMetadata(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, meta)
	}
	_, err = fmt.Fprintf(w, "Renamed session %s to %q\n", sessionID, meta.Title)
	return err
}

func writeSessionQueue(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID string, asJSON bool) error {
	queue, err := supermansession.PromptQueue(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, queue)
	}
	if len(queue) == 0 {
		_, err := fmt.Fprintln(w, "Prompt queue is empty")
		return err
	}
	for i, prompt := range queue {
		fmt.Fprintf(w, "%d. %s\n", i+1, prompt.Content)
	}
	return nil
}

func resolveSessionID(svc adksession.Service, cfg *config.Config, userID, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("session id is required")
	}
	sessions := supermansession.ListSessionMetadata(svc, cfg.Session.AppName, userID)
	var matches []string
	for _, meta := range sessions {
		if meta.SessionID == id || strings.HasPrefix(meta.SessionID, id) {
			matches = append(matches, meta.SessionID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("session not found: %s", id)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("session id %q is ambiguous: %s", id, strings.Join(matches, ", "))
	}
}

func writeSessionQueueAdd(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID, prompt string, asJSON bool) error {
	queued, err := supermansession.EnqueuePrompt(svc, cfg.Session.AppName, userID, sessionID, prompt)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, queued)
	}
	_, err = fmt.Fprintf(w, "Queued prompt %s for session %s\n", queued.ID, sessionID)
	return err
}

func writeSessionQueueClear(w io.Writer, svc adksession.Service, cfg *config.Config, userID, sessionID string, asJSON bool) error {
	cleared, err := supermansession.ClearPromptQueue(svc, cfg.Session.AppName, userID, sessionID)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, map[string]any{
			"session_id": sessionID,
			"cleared":    cleared,
		})
	}
	_, err = fmt.Fprintf(w, "Cleared %d queued prompt(s) for session %s\n", cleared, sessionID)
	return err
}

func writeSessionStorage(w io.Writer, svc adksession.Service, _ *config.Config, asJSON bool) error {
	stats, err := supermansession.StorageStatsFor(svc)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, stats)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tCOUNT\tBYTES")
	fmt.Fprintf(tw, "sessions\t%d\t%d\n", stats.Sessions, stats.SessionBytes)
	fmt.Fprintf(tw, "messages\t%d\t\n", stats.Messages)
	fmt.Fprintf(tw, "files\t%d\t\n", stats.Files)
	fmt.Fprintf(tw, "file revisions\t%d\t\n", stats.FileRevisions)
	fmt.Fprintf(tw, "prompt queue\t%d\t\n", stats.PromptQueue)
	fmt.Fprintf(tw, "references\t%d\t\n", stats.References)
	fmt.Fprintf(tw, "snapshots\t%d\t%d\n", stats.SnapshotCount, stats.SnapshotBytes)
	fmt.Fprintf(tw, "referenced snapshots\t%d\t%d\n", stats.ReferencedSnapshotCount, stats.ReferencedSnapshotBytes)
	fmt.Fprintf(tw, "orphan snapshots\t%d\t%d\n", stats.OrphanSnapshotCount, stats.OrphanSnapshotBytes)
	return tw.Flush()
}

func writeSessionStorageGC(w io.Writer, svc adksession.Service, _ *config.Config, dryRun bool, asJSON bool) error {
	result, err := supermansession.CleanupOrphanSnapshots(svc, dryRun)
	if err != nil {
		return err
	}
	if asJSON {
		return writeJSON(w, result)
	}
	if dryRun {
		_, err = fmt.Fprintf(w, "Dry run: found %d orphan snapshot(s), %d bytes reclaimable\n", result.Removed, result.RemovedBytes)
		return err
	}
	_, err = fmt.Fprintf(w, "Removed %d orphan snapshot(s), reclaimed %d bytes\n", result.Removed, result.RemovedBytes)
	return err
}

func queuePromptInput(args []string) (string, error) {
	switch {
	case queueFile != "":
		data, err := os.ReadFile(queueFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		return string(data), nil
	case queuePrompt != "":
		return queuePrompt, nil
	case len(args) > 1:
		return args[1], nil
	default:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		if len(data) == 0 {
			return "", fmt.Errorf("no prompt provided: use --prompt, --file, arg, or stdin")
		}
		return string(data), nil
	}
}

func indentLines(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseMessageRoles(value string) ([]supermansession.MessageRole, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	roles := make([]supermansession.MessageRole, 0, len(parts))
	for _, part := range parts {
		switch role := supermansession.MessageRole(strings.ToLower(strings.TrimSpace(part))); role {
		case supermansession.MessageUser, supermansession.MessageAssistant, supermansession.MessageTool, supermansession.MessageError:
			roles = append(roles, role)
		default:
			return nil, fmt.Errorf("unsupported message role %q", part)
		}
	}
	return roles, nil
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
