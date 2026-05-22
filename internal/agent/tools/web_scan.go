package tools

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type webScanInput struct {
	URL string `json:"url" jsonschema:"URL of the web page to fetch"`
}

type webScanOutput struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	StatusCode  int    `json:"status_code"`
	Truncated   bool   `json:"truncated"`
}

func newWebScanTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input webScanInput) (webScanOutput, error) {
		return scanWeb(deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "web_scan",
		Description: "Fetch a web page and return its text content",
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
