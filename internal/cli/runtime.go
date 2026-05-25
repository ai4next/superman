package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ai4next/superman/internal/global"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
)

var (
	runtimeJSON    bool
	runtimeSession string
	runtimeRun     string
	runtimeTypes   string
	runtimeLimit   int
)

var runtimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "Inspect runtime audit events",
}

var runtimeEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "List runtime audit events",
	RunE: func(cmd *cobra.Command, args []string) error {
		events, err := readRuntimeAuditEvents()
		if err != nil {
			return err
		}
		return writeRuntimeEvents(os.Stdout, events, runtimeJSON)
	},
}

var runtimeSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summarize runtime audit events",
	RunE: func(cmd *cobra.Command, args []string) error {
		events, err := readRuntimeAuditEvents()
		if err != nil {
			return err
		}
		return writeRuntimeSummary(os.Stdout, supermanruntime.SummarizeAuditEvents(events), runtimeJSON)
	},
}

func init() {
	runtimeCmd.PersistentFlags().BoolVar(&runtimeJSON, "json", false, "print JSON")
	runtimeCmd.PersistentFlags().StringVar(&runtimeSession, "session", "", "filter by session id")
	runtimeCmd.PersistentFlags().StringVar(&runtimeRun, "run", "", "filter by run id")
	runtimeCmd.PersistentFlags().StringVar(&runtimeTypes, "type", "", "filter by comma-separated event type")
	runtimeCmd.PersistentFlags().IntVar(&runtimeLimit, "limit", 100, "max events to read after filtering")
	runtimeCmd.AddCommand(runtimeEventsCmd, runtimeSummaryCmd)
}

func readRuntimeAuditEvents() ([]supermanruntime.Event, error) {
	types, err := parseRuntimeEventTypes(runtimeTypes)
	if err != nil {
		return nil, err
	}
	return supermanruntime.ReadAuditLog(global.RuntimeEventsPath(), supermanruntime.AuditFilter{
		SessionID: strings.TrimSpace(runtimeSession),
		RunID:     strings.TrimSpace(runtimeRun),
		Types:     types,
		Limit:     runtimeLimit,
	})
}

func writeRuntimeEvents(w io.Writer, events []supermanruntime.Event, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	}
	if len(events) == 0 {
		_, err := fmt.Fprintln(w, "No runtime events")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tSESSION\tRUN\tTYPE\tDETAIL")
	for _, event := range events {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			event.At.Format(timeFormat),
			event.SessionID,
			event.RunID,
			event.Type,
			runtimeEventDetail(event),
		)
	}
	return tw.Flush()
}

func writeRuntimeSummary(w io.Writer, summary supermanruntime.AuditSummary, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	fmt.Fprintf(w, "Events: %d\n", summary.Events)
	if !summary.FirstAt.IsZero() {
		fmt.Fprintf(w, "Window: %s -> %s (%s)\n", summary.FirstAt.Format(timeFormat), summary.LastAt.Format(timeFormat), summary.Duration)
	}
	if summary.Errors > 0 {
		fmt.Fprintf(w, "Errors: %d\n", summary.Errors)
	}
	writeRuntimeCountMap(w, "Types", eventTypeCountMap(summary.ByType))
	writeRuntimeCountMap(w, "Sessions", summary.Sessions)
	writeRuntimeCountMap(w, "Runs", summary.Runs)
	writeRuntimeCountMap(w, "Tools", summary.Tools)
	return nil
}

func parseRuntimeEventTypes(value string) ([]supermanruntime.EventType, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	out := make([]supermanruntime.EventType, 0, len(parts))
	for _, part := range parts {
		typ := supermanruntime.EventType(strings.TrimSpace(part))
		if typ == "" {
			continue
		}
		out = append(out, typ)
	}
	return out, nil
}

func runtimeEventDetail(event supermanruntime.Event) string {
	switch event.Type {
	case supermanruntime.EventTextDelta:
		return singleLine(event.Text)
	case supermanruntime.EventToolCallStarted:
		return firstNonEmpty(event.ToolName, event.ToolID) + " " + singleLine(event.Args)
	case supermanruntime.EventToolCallFinished:
		return strings.TrimSpace(firstNonEmpty(event.ToolName, event.ToolID) + " " + event.Status + " " + singleLine(event.Result))
	case supermanruntime.EventPermissionRequested, supermanruntime.EventPermissionGranted, supermanruntime.EventPermissionDenied:
		return strings.TrimSpace(firstNonEmpty(event.ToolName, event.ToolID) + " " + event.Status)
	case supermanruntime.EventRunFailed, supermanruntime.EventEvolutionFailed:
		return event.Error
	case supermanruntime.EventEvolutionFinished:
		return event.Path
	case supermanruntime.EventSessionCompacted:
		return fmt.Sprintf("messages=%d", event.Count)
	default:
		return firstNonEmpty(event.Error, event.ToolName, event.Path, event.Role)
	}
}

func writeRuntimeCountMap(w io.Writer, title string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(w, "  %s: %d\n", key, counts[key])
	}
}

func eventTypeCountMap(in map[supermanruntime.EventType]int) map[string]int {
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[string(key)] = value
	}
	return out
}
