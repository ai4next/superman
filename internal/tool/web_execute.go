package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type webExecuteInput struct {
	Script      string `json:"script" jsonschema:"JavaScript to run in the page. Return a value from an async IIFE when needed."`
	URL         string `json:"url,omitempty" jsonschema:"Optional HTTP(S) URL to navigate before executing script"`
	Timeout     string `json:"timeout,omitempty" jsonschema:"Optional timeout like 10s, 2m"`
	SwitchTabID string `json:"switch_tab_id,omitempty" jsonschema:"Optional tab id returned by web_scan"`
	TabID       string `json:"tab_id,omitempty" jsonschema:"Alias for switch_tab_id"`
	NoMonitor   bool   `json:"no_monitor,omitempty" jsonschema:"Skip DOM change monitoring"`
	SaveToFile  string `json:"save_to_file,omitempty" jsonschema:"Save long JavaScript return value to this file"`
	NewTab      bool   `json:"new_tab,omitempty" jsonschema:"Open url in a new browser tab before executing"`
}

type webExecuteOutput struct {
	Result   string    `json:"result"`
	JSReturn any       `json:"js_return,omitempty"`
	Status   string    `json:"status,omitempty"`
	TabID    string    `json:"tab_id,omitempty"`
	Diff     string    `json:"diff,omitempty"`
	NewTabs  []tabInfo `json:"newTabs,omitempty"`
	URL      string    `json:"url,omitempty"`
	Title    string    `json:"title,omitempty"`
	Tabs     []tabInfo `json:"tabs,omitempty"`
	SavedTo  string    `json:"saved_to,omitempty"`
}

func newWebExecuteTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input webExecuteInput) (webExecuteOutput, error) {
		return executeBrowserJS(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "web_execute",
		Description: "Run JavaScript in a Chrome browser using ChromeDP. Can optionally navigate to a URL first.",
	}, handler)
	return t
}

var sharedBrowser = &browserSession{}

type browserSession struct {
	mu          sync.Mutex
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	rootCtx     context.Context
	cancelRoot  context.CancelFunc
	tabs        map[target.ID]context.Context
	cancelTabs  map[target.ID]context.CancelFunc
	defaultID   target.ID
	cfgKey      string
}

func executeBrowserJS(deps Dependencies, input webExecuteInput) (webExecuteOutput, error) {
	script, err := resolveBrowserScript(deps, input.Script)
	if err != nil {
		return webExecuteOutput{}, err
	}
	input.Script = script
	if strings.TrimSpace(input.Script) == "" {
		return webExecuteOutput{}, fmt.Errorf("script is required")
	}

	timeout := deps.Config.Tools.WebExecute.Timeout.AsDuration()
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	if input.Timeout != "" {
		parsed, err := time.ParseDuration(input.Timeout)
		if err != nil {
			return webExecuteOutput{}, fmt.Errorf("invalid timeout %q: %w", input.Timeout, err)
		}
		timeout = parsed
	}

	root, err := sharedBrowser.root(deps)
	if err != nil {
		return webExecuteOutput{}, err
	}
	beforeTabs, _ := browserTabs(root)
	beforeIDs := tabIDSet(beforeTabs)

	ctx, targetID, err := sharedBrowser.tabContext(deps, input.targetID(), input.URL, input.NewTab)
	if err != nil {
		return webExecuteOutput{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var title, currentURL, beforeHTML, afterHTML string
	tasks := []chromedp.Action{}
	if input.URL != "" {
		if !isAllowedBrowserURL(input.URL) {
			return webExecuteOutput{}, fmt.Errorf("unsupported URL %q: only http and https URLs are allowed", input.URL)
		}
		tasks = append(tasks,
			chromedp.Navigate(input.URL),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
	}
	if !input.NoMonitor {
		tasks = append(tasks, chromedp.OuterHTML("html", &beforeHTML, chromedp.ByQuery))
	}

	var raw json.RawMessage
	tasks = append(tasks,
		chromedp.Evaluate(wrapBrowserScript(input.Script), &raw),
		chromedp.Title(&title),
		chromedp.Location(&currentURL),
	)
	if !input.NoMonitor {
		tasks = append(tasks, chromedp.OuterHTML("html", &afterHTML, chromedp.ByQuery))
	}

	if err := chromedp.Run(runCtx, tasks...); err != nil {
		return webExecuteOutput{}, fmt.Errorf("browser execution failed: %w", err)
	}

	tabs, _ := browserTabs(root)
	markActive(tabs, targetID)
	newTabs := newBrowserTabs(beforeIDs, tabs)
	diff := ""
	if !input.NoMonitor {
		diff = browserDiffSummary(beforeHTML, afterHTML, len(newTabs))
	}
	jsReturn := decodeJSONResult(raw)
	savedTo := ""
	if input.SaveToFile != "" {
		path, err := browserToolPath(deps, input.SaveToFile)
		if err != nil {
			return webExecuteOutput{}, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return webExecuteOutput{}, err
		}
		if err := os.WriteFile(path, []byte(formatJSONResult(raw)), 0644); err != nil {
			return webExecuteOutput{}, err
		}
		savedTo = path
		if s, ok := jsReturn.(string); ok && len(s) > 170 {
			jsReturn = s[:170] + fmt.Sprintf("\n\n[已保存完整内容到 %s]", path)
		}
	}

	return webExecuteOutput{
		Result:   formatJSONResult(raw),
		JSReturn: jsReturn,
		Status:   "success",
		TabID:    string(targetID),
		Diff:     diff,
		NewTabs:  newTabs,
		URL:      currentURL,
		Title:    title,
		Tabs:     tabs,
		SavedTo:  savedTo,
	}, nil
}

func resolveBrowserScript(deps Dependencies, script string) (string, error) {
	path, err := browserToolPath(deps, strings.TrimSpace(script))
	if err == nil {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", fmt.Errorf("read script file: %w", readErr)
			}
			return string(data), nil
		}
	}
	return script, nil
}

func browserToolPath(deps Dependencies, rawPath string) (string, error) {
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(rawPath) {
		return filepath.Clean(rawPath), nil
	}
	base := "."
	if deps.Config != nil && deps.Config.Workspace != "" {
		base = deps.Config.Workspace
	}
	return filepath.Abs(filepath.Join(base, rawPath))
}

func (input webExecuteInput) targetID() target.ID {
	if input.SwitchTabID != "" {
		return target.ID(input.SwitchTabID)
	}
	if input.TabID != "" {
		return target.ID(input.TabID)
	}
	return ""
}

func (s *browserSession) root(deps Dependencies) (context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rootLocked(deps)
}

func (s *browserSession) rootLocked(deps Dependencies) (context.Context, error) {
	if deps.Config == nil {
		return nil, fmt.Errorf("config is required")
	}

	cfg := deps.Config.Tools.WebExecute
	key := strings.Join([]string{
		cfg.RemoteDebuggingURL,
		cfg.UserDataDir,
		cfg.BrowserPath,
		fmt.Sprintf("%t", cfg.Headless),
	}, "\x00")
	if s.rootCtx != nil && s.cfgKey == key {
		return s.rootCtx, nil
	}
	if s.cancelRoot != nil {
		s.cancelRoot()
	}
	if s.cancelAlloc != nil {
		s.cancelAlloc()
	}
	s.tabs = map[target.ID]context.Context{}
	s.cancelTabs = map[target.ID]context.CancelFunc{}
	s.defaultID = ""

	base := context.Background()
	if cfg.RemoteDebuggingURL != "" {
		allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(base, cfg.RemoteDebuggingURL)
		ctx, cancelCtx := chromedp.NewContext(allocCtx)
		s.allocCtx, s.cancelAlloc = allocCtx, cancelAlloc
		s.rootCtx, s.cancelRoot, s.cfgKey = ctx, cancelCtx, key
		return ctx, nil
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", cfg.Headless),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.UserDataDir(cfg.UserDataDir),
	)
	if cfg.BrowserPath != "" {
		opts = append(opts, chromedp.ExecPath(cfg.BrowserPath))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(base, opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	s.allocCtx, s.cancelAlloc = allocCtx, cancelAlloc
	s.rootCtx, s.cancelRoot, s.cfgKey = ctx, cancelCtx, key
	return ctx, nil
}

func (s *browserSession) tabContext(deps Dependencies, requested target.ID, navURL string, newTab bool) (context.Context, target.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	root, err := s.rootLocked(deps)
	if err != nil {
		return nil, "", err
	}
	if err := chromedp.Run(root); err != nil {
		return nil, "", fmt.Errorf("start browser: %w", err)
	}

	if newTab {
		if navURL == "" {
			navURL = "about:blank"
		}
		if !isAllowedBrowserURL(navURL) && navURL != "about:blank" {
			return nil, "", fmt.Errorf("unsupported URL %q: only http and https URLs are allowed", navURL)
		}
		created, err := target.CreateTarget(navURL).WithFocus(true).Do(root)
		if err != nil {
			return nil, "", fmt.Errorf("create browser tab: %w", err)
		}
		requested = created
		s.defaultID = created
	}

	if requested == "" {
		if s.defaultID != "" {
			requested = s.defaultID
		} else if id := chromedp.FromContext(root).Target.TargetID; id != "" {
			requested = id
		} else {
			targets, err := browserTabs(root)
			if err != nil {
				return nil, "", err
			}
			if len(targets) > 0 {
				requested = target.ID(targets[0].ID)
			}
		}
	}
	if requested == "" {
		return nil, "", fmt.Errorf("no browser tab is available")
	}
	s.defaultID = requested

	if ctx, ok := s.tabs[requested]; ok {
		_ = target.ActivateTarget(requested).Do(root)
		return ctx, requested, nil
	}
	ctx, cancel := chromedp.NewContext(root, chromedp.WithTargetID(requested))
	s.tabs[requested] = ctx
	s.cancelTabs[requested] = cancel
	_ = target.ActivateTarget(requested).Do(root)
	return ctx, requested, nil
}

func (s *browserSession) defaultTargetID() target.ID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.defaultID
}

func wrapBrowserScript(script string) string {
	return `(async () => {
` + script + `
})()`
}

func formatJSONResult(raw json.RawMessage) string {
	v := decodeJSONResult(raw)
	if v == nil {
		return "null"
	}
	if s, ok := v.(string); ok {
		return s
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(pretty)
}

func decodeJSONResult(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

func tabIDSet(tabs []tabInfo) map[string]bool {
	ids := make(map[string]bool, len(tabs))
	for _, tab := range tabs {
		ids[tab.ID] = true
	}
	return ids
}

func newBrowserTabs(before map[string]bool, after []tabInfo) []tabInfo {
	var out []tabInfo
	for _, tab := range after {
		if !before[tab.ID] {
			out = append(out, tab)
		}
	}
	return out
}

func browserDiffSummary(beforeHTML, afterHTML string, newTabs int) string {
	if beforeHTML == "" || afterHTML == "" {
		return "页面变化监控不可用"
	}
	if beforeHTML == afterHTML && newTabs == 0 {
		return "DOM变化量: 0 (页面无变化)"
	}
	return fmt.Sprintf("DOM变化量: %d", absInt(len(afterHTML)-len(beforeHTML)))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func isAllowedBrowserURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}
