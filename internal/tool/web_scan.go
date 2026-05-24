package tool

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type webScanInput struct {
	URL         string `json:"url,omitempty" jsonschema:"Optional HTTP(S) URL. If empty, scan the current browser tab."`
	TabsOnly    bool   `json:"tabs_only,omitempty" jsonschema:"Show browser tab list only, no page content"`
	SwitchTabID string `json:"switch_tab_id,omitempty" jsonschema:"Optional Chrome target/tab id to switch to before scanning"`
	TextOnly    bool   `json:"text_only,omitempty" jsonschema:"Return plain visible text instead of HTML"`
	MaxLen      int    `json:"maxlen,omitempty" jsonschema:"Maximum content length"`
}

type webScanOutput struct {
	Title       string           `json:"title"`
	Content     string           `json:"content"`
	ContentType string           `json:"content_type"`
	StatusCode  int              `json:"status_code"`
	Truncated   bool             `json:"truncated"`
	URL         string           `json:"url,omitempty"`
	Tabs        []tabInfo        `json:"tabs,omitempty"`
	Status      string           `json:"status,omitempty"`
	Msg         string           `json:"msg,omitempty"`
	Metadata    *webScanMetadata `json:"metadata,omitempty"`
}

type webScanMetadata struct {
	TabsCount int       `json:"tabs_count"`
	Tabs      []tabInfo `json:"tabs"`
	ActiveTab string    `json:"active_tab"`
}

type tabInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Attached bool   `json:"attached"`
	Active   bool   `json:"active,omitempty"`
}

func browserTabs(ctx context.Context) ([]tabInfo, error) {
	targets, err := target.GetTargets().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list browser tabs: %w", err)
	}
	tabs := make([]tabInfo, 0, len(targets))
	for _, t := range targets {
		if t == nil || t.Type != "page" {
			continue
		}
		if !isScriptableBrowserURL(t.URL) {
			continue
		}
		tabs = append(tabs, tabInfo{
			ID:       string(t.TargetID),
			Title:    t.Title,
			URL:      truncateTabURL(t.URL),
			Attached: t.Attached,
		})
	}
	return tabs, nil
}

func truncateTabURL(rawURL string) string {
	const max = 50
	if len(rawURL) <= max {
		return rawURL
	}
	return rawURL[:max] + "..."
}

func markActive(tabs []tabInfo, id target.ID) {
	for i := range tabs {
		tabs[i].Active = tabs[i].ID == string(id)
	}
}

func isScriptableBrowserURL(rawURL string) bool {
	if rawURL == "" || rawURL == "about:blank" {
		return true
	}
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "file://")
}

func newWebScanTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input webScanInput) (webScanOutput, error) {
		return scanWeb(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "web_scan",
		Description: "Get browser tab list and current page content, or fetch an HTTP(S) page directly when url is provided.",
	}, handler)
	return t
}

// isPrivateHost resolves a hostname and returns true if any resolved IP
// falls within private or reserved address ranges.
func isPrivateHost(host string) bool {
	addrs, err := net.LookupIP(host)
	if err != nil {
		return true // treat resolution failure as unsafe
	}

	for _, addr := range addrs {
		if isPrivateIP(addr) {
			return true
		}
	}
	return false
}

// isPrivateIP checks whether an IP address belongs to a private or reserved range.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	if ip.IsLinkLocalUnicast() {
		return true
	}
	if ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsInterfaceLocalMulticast() {
		return true
	}
	if ip.IsUnspecified() {
		return true
	}

	// Private network ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"fc00::/7",
		"::1/128",
	}

	for _, cidr := range privateRanges {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func scanWeb(deps Dependencies, input webScanInput) (webScanOutput, error) {
	if input.URL == "" || input.TabsOnly || input.SwitchTabID != "" {
		return scanBrowser(deps, input)
	}
	if input.URL != "" && deps.Config != nil && deps.Config.Tools.WebExecute.Enabled {
		return scanBrowser(deps, input)
	}
	return scanHTTP(deps, input)
}

func scanHTTP(deps Dependencies, input webScanInput) (webScanOutput, error) {
	timeout := deps.Config.Tools.WebScan.Timeout.AsDuration()
	client := &http.Client{Timeout: timeout}

	parsedURL, err := url.Parse(input.URL)
	if err != nil {
		return webScanOutput{}, fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return webScanOutput{}, fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", parsedURL.Scheme)
	}
	host := parsedURL.Hostname()
	if host == "" {
		return webScanOutput{}, fmt.Errorf("invalid URL: no host found")
	}
	if isPrivateHost(host) {
		return webScanOutput{}, fmt.Errorf("URL resolves to a private or reserved IP address and is not allowed for safety")
	}

	resp, err := client.Get(input.URL)
	if err != nil {
		return webScanOutput{}, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	const maxSize = 1 * 1024 * 1024 // 1MB max response
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return webScanOutput{}, fmt.Errorf("read body failed: %w", err)
	}

	rawHTML := string(body)
	truncated := len(body) >= maxSize

	title := extractTitleFromHTML(rawHTML)

	content := stripHTMLTags(rawHTML)
	content = collapseWhitespace(content)

	const maxOutput = 50000
	if len(content) > maxOutput {
		content = content[:maxOutput]
		truncated = true
	}

	if title == "" {
		title = extractTitle(content)
	}

	return webScanOutput{
		Title:       title,
		Content:     content,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
		Truncated:   truncated,
	}, nil
}

func scanBrowser(deps Dependencies, input webScanInput) (webScanOutput, error) {
	root, err := sharedBrowser.root(deps)
	if err != nil {
		return webScanOutput{}, err
	}
	if err := chromedp.Run(root); err != nil {
		return webScanOutput{}, fmt.Errorf("start browser: %w", err)
	}

	tabs, err := browserTabs(root)
	if err != nil {
		return webScanOutput{}, err
	}
	if input.TabsOnly {
		active := string(sharedBrowser.defaultTargetID())
		return webScanOutput{
			Status: "success",
			Metadata: &webScanMetadata{
				TabsCount: len(tabs),
				Tabs:      tabs,
				ActiveTab: active,
			},
			Tabs: tabs,
		}, nil
	}

	ctx, id, err := sharedBrowser.tabContext(deps, target.ID(input.SwitchTabID), input.URL, input.URL != "")
	if err != nil {
		return webScanOutput{}, err
	}
	timeout := deps.Config.Tools.WebScan.Timeout.AsDuration()
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var title, currentURL, content string
	actions := []chromedp.Action{}
	if input.URL != "" {
		if !isAllowedBrowserURL(input.URL) {
			return webScanOutput{}, fmt.Errorf("unsupported URL %q: only http and https URLs are allowed", input.URL)
		}
		actions = append(actions, chromedp.Navigate(input.URL), chromedp.WaitReady("body", chromedp.ByQuery))
	}
	actions = append(actions, chromedp.Evaluate(wrapBrowserScript(genericAgentScanScript(input.TextOnly)), &content))
	actions = append(actions, chromedp.Title(&title), chromedp.Location(&currentURL))
	if err := chromedp.Run(runCtx, actions...); err != nil {
		return webScanOutput{
			Status: "error",
			Msg:    err.Error(),
		}, nil
	}

	maxOutput := 35000
	if input.MaxLen > 0 {
		maxOutput = input.MaxLen
	}
	if input.TextOnly {
		content = normalizeTextOnlyContent(content)
		content = genericAgentSmartFormat(content, maxOutput/3, "\n\n[omitted long content]\n\n")
	}
	truncated := false
	if len(content) > maxOutput {
		content = content[:maxOutput]
		truncated = true
	}
	tabs, _ = browserTabs(root)
	markActive(tabs, id)
	return webScanOutput{
		Title:       title,
		Content:     content,
		ContentType: "text/plain; browser=chromedp",
		StatusCode:  0,
		Truncated:   truncated,
		URL:         currentURL,
		Tabs:        tabs,
		Status:      "success",
		Metadata: &webScanMetadata{
			TabsCount: len(tabs),
			Tabs:      tabs,
			ActiveTab: string(id),
		},
	}, nil
}

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(c)
		}
	}
	return result.String()
}

func collapseWhitespace(s string) string {
	var result strings.Builder
	prevSpace := false
	for _, c := range s {
		if c == '\n' || c == '\t' || c == '\r' {
			c = ' '
		}
		if c == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		result.WriteRune(c)
	}
	return strings.TrimSpace(result.String())
}

func extractTitle(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func extractTitleFromHTML(html string) string {
	// Extract text between <title> and </title>
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title>")
	if start == -1 {
		return ""
	}
	start += 7 // len("<title>")
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	title := strings.TrimSpace(html[start : start+end])
	if len(title) > 200 {
		title = title[:200]
	}
	return title
}
