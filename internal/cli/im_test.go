package cli

import (
	"testing"

	"github.com/ai4next/superman/internal/config"
)

func TestIMConfigFromApp(t *testing.T) {
	cfg := &config.Config{
		IM: config.IMConfig{
			Platforms: []config.IMPlatformConfig{{
				Name:    "telegram",
				Enabled: true,
				Options: map[string]interface{}{
					"token":      "secret",
					"allow_from": "u1",
				},
			}},
		},
	}

	got := imConfigFromApp(cfg)
	if len(got.Platforms) != 1 {
		t.Fatalf("platform count = %d, want 1", len(got.Platforms))
	}
	if got.Platforms[0].Name != "telegram" || !got.Platforms[0].Enabled {
		t.Fatalf("platform = %+v", got.Platforms[0])
	}
	if got.Platforms[0].Options["token"] != "secret" {
		t.Fatalf("token option = %#v, want secret", got.Platforms[0].Options["token"])
	}
}
