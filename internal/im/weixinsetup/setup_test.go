package weixinsetup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveQRCodeImageWritesPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qr.png")
	if err := SaveQRCodeImage("https://example.com/login", path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || !bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("saved file is not a PNG")
	}
}

func TestPrintTerminalQRCodeWritesBlocks(t *testing.T) {
	var out bytes.Buffer
	PrintTerminalQRCode(&out, "https://example.com/login")
	got := out.String()
	if !strings.ContainsAny(got, "█▀▄") {
		t.Fatalf("terminal QR output does not contain block characters: %q", got)
	}
}
