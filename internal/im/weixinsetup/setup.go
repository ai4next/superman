package weixinsetup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rsc.io/qr"
)

const (
	DefaultAPIURL  = "https://ilinkai.weixin.qq.com"
	DefaultBotType = "3"
	PollTimeout    = 35 * time.Second
	MaxQRRefresh   = 3
)

type QRLoginOptions struct {
	APIBaseURL string
	RouteTag   string
	BotType    string
	Timeout    time.Duration
	QRImage    string
	Debug      bool
	Out        io.Writer
	Err        io.Writer
}

type QRLoginResult struct {
	BotToken    string
	IlinkBotID  string
	BaseURL     string
	IlinkUserID string
}

type botQRResponse struct {
	QRCode           string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

type qrStatusResponse struct {
	Status      string `json:"status"`
	BotToken    string `json:"bot_token"`
	IlinkBotID  string `json:"ilink_bot_id"`
	BaseURL     string `json:"baseurl"`
	IlinkUserID string `json:"ilink_user_id"`
}

func RunQRLoginFlow(ctx context.Context, opts QRLoginOptions) (*QRLoginResult, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	if opts.Timeout < time.Second {
		opts.Timeout = 480 * time.Second
	}
	botType := strings.TrimSpace(opts.BotType)
	if botType == "" {
		botType = DefaultBotType
	}

	qrPayload, err := FetchBotQRCode(ctx, opts.APIBaseURL, botType, opts.RouteTag, opts.Debug, opts.Err)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(qrPayload.QRCodeImgContent) == "" {
		return nil, fmt.Errorf("empty qrcode_img_content from server")
	}

	qrURL := strings.TrimSpace(qrPayload.QRCodeImgContent)
	fmt.Fprintln(opts.Out, "Use WeChat to scan the QR code below, or open the URL:")
	fmt.Fprintf(opts.Out, "URL: %s\n\n", qrURL)
	PrintTerminalQRCode(opts.Out, qrURL)
	if opts.QRImage != "" {
		if err := SaveQRCodeImage(qrURL, opts.QRImage); err != nil {
			fmt.Fprintf(opts.Err, "Warning: failed to save QR image: %v\n", err)
		} else {
			fmt.Fprintf(opts.Out, "QR code saved to: %s\n\n", opts.QRImage)
		}
	}

	deadline := time.Now().Add(opts.Timeout)
	qrKey := qrPayload.QRCode
	refreshCount := 1
	scannedPrinted := false
	for time.Now().Before(deadline) {
		status, err := PollQRStatus(ctx, opts.APIBaseURL, qrKey, opts.RouteTag, opts.Debug, opts.Err)
		if err != nil {
			return nil, err
		}
		switch status.Status {
		case "wait", "":
			time.Sleep(time.Second)
		case "scaned":
			if !scannedPrinted {
				fmt.Fprintln(opts.Out, "\nScanned. Confirm login on your phone...")
				scannedPrinted = true
			}
			time.Sleep(time.Second)
		case "expired":
			refreshCount++
			if refreshCount > MaxQRRefresh {
				return nil, fmt.Errorf("QR code expired too many times; retry setup")
			}
			fmt.Fprintf(opts.Out, "\nQR code expired; refreshing (%d/%d)...\n", refreshCount, MaxQRRefresh)
			newQR, err := FetchBotQRCode(ctx, opts.APIBaseURL, botType, opts.RouteTag, opts.Debug, opts.Err)
			if err != nil {
				return nil, fmt.Errorf("refresh QR: %w", err)
			}
			qrKey = newQR.QRCode
			scannedPrinted = false
			newURL := strings.TrimSpace(newQR.QRCodeImgContent)
			if newURL != "" {
				fmt.Fprintln(opts.Out, "Scan the new QR code:")
				fmt.Fprintf(opts.Out, "URL: %s\n\n", newURL)
				PrintTerminalQRCode(opts.Out, newURL)
			}
			time.Sleep(time.Second)
		case "confirmed":
			if strings.TrimSpace(status.IlinkBotID) == "" {
				return nil, fmt.Errorf("login confirmed but ilink_bot_id missing")
			}
			if strings.TrimSpace(status.BotToken) == "" {
				return nil, fmt.Errorf("login confirmed but bot_token missing")
			}
			fmt.Fprintln(opts.Out, "\nWeixin login confirmed.")
			return &QRLoginResult{
				BotToken:    strings.TrimSpace(status.BotToken),
				IlinkBotID:  strings.TrimSpace(status.IlinkBotID),
				BaseURL:     strings.TrimSpace(status.BaseURL),
				IlinkUserID: strings.TrimSpace(status.IlinkUserID),
			}, nil
		default:
			time.Sleep(time.Second)
		}
	}
	return nil, fmt.Errorf("timed out waiting for Weixin QR login")
}

func FetchBotQRCode(ctx context.Context, apiBase, botType, routeTag string, debug bool, debugOut io.Writer) (*botQRResponse, error) {
	base := strings.TrimRight(apiBase, "/") + "/"
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	u = u.JoinPath("ilink", "bot", "get_bot_qrcode")
	q := u.Query()
	q.Set("bot_type", botType)
	u.RawQuery = q.Encode()
	raw, err := httpGet(ctx, u.String(), routeTag, debug, debugOut)
	if err != nil {
		return nil, fmt.Errorf("get_bot_qrcode: %w", err)
	}
	var out botQRResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("get_bot_qrcode json: %w", err)
	}
	return &out, nil
}

func PollQRStatus(ctx context.Context, apiBase, qrKey, routeTag string, debug bool, debugOut io.Writer) (*qrStatusResponse, error) {
	base := strings.TrimRight(apiBase, "/") + "/"
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	u = u.JoinPath("ilink", "bot", "get_qrcode_status")
	q := u.Query()
	q.Set("qrcode", qrKey)
	u.RawQuery = q.Encode()

	pollCtx, cancel := context.WithTimeout(ctx, PollTimeout+2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pollCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("iLink-App-ClientVersion", "1")
	if routeTag != "" {
		req.Header.Set("SKRouteTag", routeTag)
	}
	client := &http.Client{Timeout: PollTimeout + 5*time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return &qrStatusResponse{Status: "wait"}, nil
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return &qrStatusResponse{Status: "wait"}, nil
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if debug && debugOut != nil {
		fmt.Fprintf(debugOut, "[debug] poll status -> %d %s\n", resp.StatusCode, truncateBody(body, 200))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get_qrcode_status http %d: %s", resp.StatusCode, truncateBody(body, 256))
	}
	var out qrStatusResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("get_qrcode_status json: %w", err)
	}
	return &out, nil
}

func httpGet(ctx context.Context, fullURL, routeTag string, debug bool, debugOut io.Writer) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	if routeTag != "" {
		req.Header.Set("SKRouteTag", routeTag)
	}
	client := &http.Client{Timeout: PollTimeout + 5*time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if debug && debugOut != nil {
		fmt.Fprintf(debugOut, "[debug] GET %s -> %d %s\n", fullURL, resp.StatusCode, truncateBody(body, 200))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncateBody(body, 256))
	}
	return body, nil
}

func truncateBody(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func PrintTerminalQRCode(w io.Writer, content string) {
	if w == nil {
		return
	}
	code, err := qr.Encode(content, qr.M)
	if err != nil {
		fmt.Fprintf(w, "(QR encode failed: %v)\n\n", err)
		return
	}
	const quiet = 2
	for y := -quiet; y < code.Size+quiet; y += 2 {
		for x := -quiet; x < code.Size+quiet; x++ {
			top := code.Black(x, y)
			bottom := code.Black(x, y+1)
			switch {
			case top && bottom:
				fmt.Fprint(w, "█")
			case top && !bottom:
				fmt.Fprint(w, "▀")
			case !top && bottom:
				fmt.Fprint(w, "▄")
			default:
				fmt.Fprint(w, " ")
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
}

func SaveQRCodeImage(content, path string) error {
	code, err := qr.Encode(content, qr.M)
	if err != nil {
		return fmt.Errorf("encode QR: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, code.PNG(), 0o644)
}
