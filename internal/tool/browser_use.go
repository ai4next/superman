package tool

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type browserUseAction string

const (
	browserUseGoToURL        browserUseAction = "go_to_url"
	browserUseWebSearch      browserUseAction = "web_search"
	browserUseClickElement   browserUseAction = "click_element"
	browserUseInputText      browserUseAction = "input_text"
	browserUseScrollDown     browserUseAction = "scroll_down"
	browserUseScrollUp       browserUseAction = "scroll_up"
	browserUseExtractContent browserUseAction = "extract_content"
	browserUseSwitchTab      browserUseAction = "switch_tab"
	browserUseOpenTab        browserUseAction = "open_tab"
	browserUseCloseTab       browserUseAction = "close_tab"
	browserUseWait           browserUseAction = "wait"
	browserUseStateAction    browserUseAction = "state"
)

type browserUseInput struct {
	Action         browserUseAction `json:"action" jsonschema:"Browser action: go_to_url, web_search, click_element, input_text, scroll_down, scroll_up, extract_content, switch_tab, open_tab, close_tab, wait, state"`
	URL            string           `json:"url,omitempty" jsonschema:"URL for go_to_url or open_tab"`
	Index          int              `json:"index,omitempty" jsonschema:"Element index for click_element or input_text"`
	Text           string           `json:"text,omitempty" jsonschema:"Text for input_text"`
	ScrollAmount   int              `json:"scroll_amount,omitempty" jsonschema:"Pixels to scroll; default 500"`
	TabID          int              `json:"tab_id,omitempty" jsonschema:"Tab index returned by browser_use state"`
	Query          string           `json:"query,omitempty" jsonschema:"Search query for web_search"`
	Goal           string           `json:"goal,omitempty" jsonschema:"Extraction goal; if empty, page is summarized as simplified content"`
	Seconds        int              `json:"seconds,omitempty" jsonschema:"Seconds to wait; default 3"`
	WithScreenshot bool             `json:"with_screenshot,omitempty" jsonschema:"Include base64 viewport screenshot in the returned state"`
}

type browserUseOutput struct {
	Output      string            `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	State       *browserUseState  `json:"state,omitempty"`
	Base64Image string            `json:"base64_image,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type browserUseState struct {
	URL                 string               `json:"url"`
	Title               string               `json:"title"`
	Tabs                []browserUseTabInfo  `json:"tabs"`
	InteractiveElements string               `json:"interactive_elements"`
	Elements            []browserUseElement  `json:"elements"`
	ScrollInfo          browserUseScrollInfo `json:"scroll_info"`
	ViewportHeight      int                  `json:"viewport_height"`
	Screenshot          string               `json:"screenshot,omitempty"`
}

type browserUseTabInfo struct {
	ID       int       `json:"id"`
	TargetID target.ID `json:"target_id"`
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	Active   bool      `json:"active,omitempty"`
}

type browserUseScrollInfo struct {
	PixelsAbove int `json:"pixels_above"`
	PixelsBelow int `json:"pixels_below"`
	TotalHeight int `json:"total_height"`
}

type browserUseElement struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	Type        string `json:"type"`
	XPath       string `json:"xpath"`
}

type browserUseSession struct {
	mu          sync.Mutex
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	rootCtx     context.Context
	cancelRoot  context.CancelFunc
	tabCtx      context.Context
	cancelTab   context.CancelFunc
	tabs        []browserUseTabInfo
	elements    []browserUseElement
	currentID   target.ID
	cfgKey      string
}

var sharedBrowserUse = &browserUseSession{}

func newBrowserUseTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input browserUseInput) (browserUseOutput, error) {
		return sharedBrowserUse.execute(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "browser_use",
		Description: "High-level independent browser control: navigation, search, element click/input, scroll, extraction, tabs, wait, and state.",
	}, handler)
	return t
}

func (s *browserUseSession) execute(deps Dependencies, input browserUseInput) (browserUseOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if deps.Config == nil {
		return browserUseOutput{}, fmt.Errorf("config is required")
	}
	if err := s.ensureStartedLocked(deps); err != nil {
		return browserUseOutput{}, err
	}

	timeout := deps.Config.Tools.BrowserUse.Timeout.AsDuration()
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	runCtx, cancel := context.WithTimeout(s.tabCtx, timeout)
	defer cancel()

	var output string
	var err error
	switch input.Action {
	case browserUseGoToURL:
		output, err = s.goToURL(runCtx, input.URL)
	case browserUseWebSearch:
		output, err = s.webSearch(runCtx, input.Query)
	case browserUseClickElement:
		output, err = s.clickElement(runCtx, input.Index)
	case browserUseInputText:
		output, err = s.inputText(runCtx, input.Index, input.Text)
	case browserUseScrollDown, browserUseScrollUp:
		output, err = s.scroll(runCtx, input.Action, input.ScrollAmount)
	case browserUseWait:
		output, err = s.wait(runCtx, input.Seconds)
	case browserUseExtractContent:
		return s.extractContent(runCtx, input)
	case browserUseSwitchTab:
		output, err = s.switchTabLocked(input.TabID)
	case browserUseOpenTab:
		output, err = s.openTabLocked(input.URL)
	case browserUseCloseTab:
		output, err = s.closeTabLocked()
	case browserUseStateAction, "":
		output = "current browser state"
	default:
		return browserUseOutput{Error: fmt.Sprintf("unknown action: %s", input.Action)}, nil
	}
	if err != nil {
		return browserUseOutput{Error: err.Error()}, nil
	}
	state, stateErr := s.stateLocked(input.WithScreenshot)
	if stateErr != nil {
		return browserUseOutput{Output: output, Error: fmt.Sprintf("failed to get browser state: %v", stateErr)}, nil
	}
	return browserUseOutput{Output: output, State: state}, nil
}

func (s *browserUseSession) ensureStartedLocked(deps Dependencies) error {
	cfg := deps.Config.Tools.BrowserUse
	key := strings.Join([]string{
		cfg.UserDataDir,
		cfg.BrowserPath,
		cfg.ProxyServer,
		fmt.Sprintf("%t", cfg.Headless),
		fmt.Sprintf("%t", cfg.DisableSecurity),
		strings.Join(cfg.ExtraChromiumArgs, "\x01"),
	}, "\x00")
	if s.rootCtx != nil && s.tabCtx != nil && s.cfgKey == key {
		return nil
	}
	s.cleanupLocked()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", cfg.Headless),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.UserDataDir(cfg.UserDataDir),
	)
	if cfg.DisableSecurity {
		opts = append(opts,
			chromedp.Flag("disable-web-security", true),
			chromedp.Flag("allow-running-insecure-content", true),
		)
	}
	for _, arg := range cfg.ExtraChromiumArgs {
		if arg != "" {
			opts = append(opts, chromedp.Flag(arg, true))
		}
	}
	if cfg.BrowserPath != "" {
		opts = append(opts, chromedp.ExecPath(cfg.BrowserPath))
	}
	if cfg.ProxyServer != "" {
		opts = append(opts, chromedp.ProxyServer(cfg.ProxyServer))
	}

	s.allocCtx, s.cancelAlloc = chromedp.NewExecAllocator(context.Background(), opts...)
	s.rootCtx, s.cancelRoot = chromedp.NewContext(s.allocCtx)
	s.tabCtx = s.rootCtx
	if err := chromedp.Run(s.rootCtx); err != nil {
		s.cleanupLocked()
		return fmt.Errorf("start browser_use browser: %w", err)
	}
	s.currentID = chromedp.FromContext(s.rootCtx).Target.TargetID
	s.cfgKey = key
	return s.refreshLocked()
}

func (s *browserUseSession) cleanupLocked() {
	if s.cancelTab != nil {
		s.cancelTab()
	}
	if s.cancelRoot != nil {
		s.cancelRoot()
	}
	if s.cancelAlloc != nil {
		s.cancelAlloc()
	}
	s.allocCtx, s.cancelAlloc = nil, nil
	s.rootCtx, s.cancelRoot = nil, nil
	s.tabCtx, s.cancelTab = nil, nil
	s.tabs = nil
	s.elements = nil
	s.currentID = ""
	s.cfgKey = ""
}

func (s *browserUseSession) goToURL(ctx context.Context, rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("url is required for go_to_url")
	}
	if !browserUseAllowedURL(rawURL) {
		return "", fmt.Errorf("unsupported URL %q: only http and https URLs are allowed", rawURL)
	}
	if err := chromedp.Run(ctx, chromedp.Navigate(rawURL), chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("failed to navigate to %s: %w", rawURL, err)
	}
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("successfully navigated to %s", rawURL), nil
}

func (s *browserUseSession) webSearch(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query is required for web_search")
	}
	searchURL := "https://duckduckgo.com/?q=" + url.QueryEscape(query)
	if err := chromedp.Run(ctx, chromedp.Navigate(searchURL), chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("failed to search web: %w", err)
	}
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return "successfully searched web for: " + query, nil
}

func (s *browserUseSession) clickElement(ctx context.Context, index int) (string, error) {
	if index < 0 || index >= len(s.elements) {
		return "", fmt.Errorf("index %d out of range", index)
	}
	el := s.elements[index]
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(el.XPath, chromedp.BySearch),
		chromedp.Click(el.XPath, chromedp.BySearch),
		chromedp.Sleep(time.Second),
	); err != nil {
		return "", fmt.Errorf("failed to click element %d: %w", index, err)
	}
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("successfully clicked element %d", index), nil
}

func (s *browserUseSession) inputText(ctx context.Context, index int, text string) (string, error) {
	if index < 0 || index >= len(s.elements) {
		return "", fmt.Errorf("index %d out of range", index)
	}
	el := s.elements[index]
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(el.XPath, chromedp.BySearch),
		chromedp.Clear(el.XPath, chromedp.BySearch),
		chromedp.SendKeys(el.XPath, text, chromedp.BySearch),
	); err != nil {
		return "", fmt.Errorf("failed to input text to element %d: %w", index, err)
	}
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("successfully input text to element %d", index), nil
}

func (s *browserUseSession) scroll(ctx context.Context, action browserUseAction, amount int) (string, error) {
	if amount == 0 {
		amount = 500
	}
	if action == browserUseScrollUp {
		amount = -amount
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf("window.scrollBy(0, %d);", amount), nil)); err != nil {
		return "", fmt.Errorf("failed to scroll: %w", err)
	}
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("successfully scrolled %d pixels", amount), nil
}

func (s *browserUseSession) wait(ctx context.Context, seconds int) (string, error) {
	if seconds <= 0 {
		seconds = 3
	}
	if err := chromedp.Run(ctx, chromedp.Sleep(time.Duration(seconds)*time.Second)); err != nil {
		return "", fmt.Errorf("failed to wait: %w", err)
	}
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("successfully waited for %d seconds", seconds), nil
}

func (s *browserUseSession) extractContent(ctx context.Context, input browserUseInput) (browserUseOutput, error) {
	var content string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.body ? document.body.innerText : document.documentElement.innerText`, &content)); err != nil {
		return browserUseOutput{Error: fmt.Sprintf("extract content failed: %v", err)}, nil
	}
	content = strings.TrimSpace(content)
	if input.Goal != "" {
		content = "Extraction goal: " + input.Goal + "\n\n" + content
	}
	if len(content) > 50000 {
		content = content[:50000] + "\n\n[TRUNCATED]"
	}
	state, err := s.stateLocked(input.WithScreenshot)
	if err != nil {
		return browserUseOutput{Output: content, Error: fmt.Sprintf("failed to get browser state: %v", err)}, nil
	}
	return browserUseOutput{Output: content, State: state}, nil
}

func (s *browserUseSession) switchTabLocked(tabID int) (string, error) {
	if tabID < 0 || tabID >= len(s.tabs) {
		return "", fmt.Errorf("tab ID %d out of range", tabID)
	}
	targetID := s.tabs[tabID].TargetID
	if s.cancelTab != nil {
		s.cancelTab()
	}
	ctx, cancel := chromedp.NewContext(s.rootCtx, chromedp.WithTargetID(targetID))
	s.tabCtx, s.cancelTab = ctx, cancel
	if err := chromedp.Run(ctx, target.ActivateTarget(targetID)); err != nil {
		return "", fmt.Errorf("failed to switch tab: %w", err)
	}
	s.currentID = targetID
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("successfully switched to tab %d", tabID), nil
}

func (s *browserUseSession) openTabLocked(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("url is required for open_tab")
	}
	if !browserUseAllowedURL(rawURL) {
		return "", fmt.Errorf("unsupported URL %q: only http and https URLs are allowed", rawURL)
	}
	created, err := target.CreateTarget(rawURL).WithFocus(true).Do(s.rootCtx)
	if err != nil {
		return "", fmt.Errorf("failed to open new tab: %w", err)
	}
	if s.cancelTab != nil {
		s.cancelTab()
	}
	ctx, cancel := chromedp.NewContext(s.rootCtx, chromedp.WithTargetID(created))
	s.tabCtx, s.cancelTab = ctx, cancel
	s.currentID = created
	if err := chromedp.Run(ctx, chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
		return "", fmt.Errorf("failed to wait for new tab: %w", err)
	}
	if err := s.refreshLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("successfully opened new tab %s", rawURL), nil
}

func (s *browserUseSession) closeTabLocked() (string, error) {
	if err := chromedp.Run(s.tabCtx, page.Close()); err != nil {
		return "", fmt.Errorf("failed to close tab: %w", err)
	}
	if err := s.refreshTabsLocked(); err != nil {
		return "", err
	}
	if len(s.tabs) == 0 {
		s.tabCtx = s.rootCtx
		s.currentID = chromedp.FromContext(s.rootCtx).Target.TargetID
		s.elements = nil
		return "successfully closed current tab", nil
	}
	if err := s.switchTabByTargetLocked(s.tabs[0].TargetID); err != nil {
		return "", err
	}
	return "successfully closed current tab", nil
}

func (s *browserUseSession) switchTabByTargetLocked(targetID target.ID) error {
	if s.cancelTab != nil {
		s.cancelTab()
	}
	ctx, cancel := chromedp.NewContext(s.rootCtx, chromedp.WithTargetID(targetID))
	s.tabCtx, s.cancelTab = ctx, cancel
	s.currentID = targetID
	if err := chromedp.Run(ctx, target.ActivateTarget(targetID)); err != nil {
		return fmt.Errorf("failed to activate tab: %w", err)
	}
	return s.refreshLocked()
}

func (s *browserUseSession) refreshLocked() error {
	if err := s.refreshTabsLocked(); err != nil {
		return err
	}
	return s.updateElementsLocked()
}

func (s *browserUseSession) refreshTabsLocked() error {
	targets, err := target.GetTargets().Do(s.rootCtx)
	if err != nil {
		return fmt.Errorf("list tabs: %w", err)
	}
	s.tabs = make([]browserUseTabInfo, 0, len(targets))
	for _, t := range targets {
		if t == nil || t.Type != "page" || !browserUseScriptableURL(t.URL) {
			continue
		}
		s.tabs = append(s.tabs, browserUseTabInfo{
			ID:       len(s.tabs),
			TargetID: t.TargetID,
			Title:    t.Title,
			URL:      t.URL,
			Active:   t.TargetID == s.currentID,
		})
	}
	return nil
}

func (s *browserUseSession) updateElementsLocked() error {
	var nodes []*cdp.Node
	if err := chromedp.Run(s.tabCtx, chromedp.Nodes("a, button, input, select, textarea, [role=button], [contenteditable=true]", &nodes, chromedp.ByQueryAll)); err != nil {
		return err
	}
	s.elements = s.elements[:0]
	for _, node := range nodes {
		visible, err := browserUseNodeVisible(s.tabCtx, node)
		if err != nil || !visible {
			continue
		}
		xpath := node.FullXPath()
		s.elements = append(s.elements, browserUseElement{
			Index:       len(s.elements),
			Description: browserUseElementDescription(node),
			Type:        node.NodeName,
			XPath:       xpath,
		})
	}
	return nil
}

func browserUseNodeVisible(ctx context.Context, node *cdp.Node) (bool, error) {
	xpath := node.FullXPath()
	var visible bool
	err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(fmt.Sprintf(`(() => {
const el = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
if (!el) return false;
const rect = el.getBoundingClientRect();
if (rect.width <= 1 || rect.height <= 1) return false;
const style = window.getComputedStyle(el);
if (style.display === 'none' || style.visibility === 'hidden' || parseFloat(style.opacity) <= 0) return false;
if (rect.bottom < 0 || rect.right < 0 || rect.top > window.innerHeight || rect.left > window.innerWidth) return false;
return true;
})()`, xpath), &visible))
	return visible, err
}

func browserUseElementDescription(node *cdp.Node) string {
	tag := strings.ToLower(node.NodeName)
	text := firstNonEmpty(
		node.AttributeValue("aria-label"),
		node.AttributeValue("title"),
		node.AttributeValue("placeholder"),
		node.AttributeValue("name"),
		node.AttributeValue("value"),
		node.AttributeValue("href"),
	)
	if text == "" {
		text = tag
	}
	if len(text) > 120 {
		text = text[:120] + "..."
	}
	switch tag {
	case "a":
		return "Link: " + text
	case "button":
		return "Button: " + text
	case "input":
		return fmt.Sprintf("Input(%s): %s", firstNonEmpty(node.AttributeValue("type"), "text"), text)
	case "select":
		return "Select Dropdown: " + text
	case "textarea":
		return "TextArea: " + text
	default:
		return tag + ": " + text
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *browserUseSession) stateLocked(withScreenshot bool) (*browserUseState, error) {
	var currentURL, title string
	var scroll struct {
		ScrollHeight float64 `json:"scrollHeight"`
		ClientHeight float64 `json:"clientHeight"`
		ScrollTop    float64 `json:"scrollTop"`
	}
	if err := chromedp.Run(s.tabCtx,
		chromedp.Location(&currentURL),
		chromedp.Title(&title),
		chromedp.Evaluate(`(() => ({
scrollHeight: document.documentElement.scrollHeight || document.body.scrollHeight || 0,
clientHeight: document.documentElement.clientHeight || window.innerHeight || 0,
scrollTop: document.documentElement.scrollTop || document.body.scrollTop || 0
}))()`, &scroll),
	); err != nil {
		return nil, err
	}
	if err := s.refreshLocked(); err != nil {
		return nil, err
	}
	var interactive strings.Builder
	for _, elem := range s.elements {
		fmt.Fprintf(&interactive, "[%d] %s\n", elem.Index, elem.Description)
	}
	state := &browserUseState{
		URL:                 currentURL,
		Title:               title,
		Tabs:                s.tabs,
		InteractiveElements: interactive.String(),
		Elements:            s.elements,
		ScrollInfo: browserUseScrollInfo{
			PixelsAbove: int(scroll.ScrollTop),
			PixelsBelow: maxInt(0, int(scroll.ScrollHeight-scroll.ClientHeight-scroll.ScrollTop)),
			TotalHeight: int(scroll.ScrollHeight),
		},
		ViewportHeight: int(scroll.ClientHeight),
	}
	if withScreenshot {
		var buf []byte
		if err := chromedp.Run(s.tabCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
			return nil, err
		}
		state.Screenshot = base64.StdEncoding.EncodeToString(buf)
	}
	return state, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func browserUseAllowedURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

func browserUseScriptableURL(rawURL string) bool {
	if rawURL == "" || rawURL == "about:blank" {
		return true
	}
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "file://")
}
