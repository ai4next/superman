package global

import (
	"sync"

	"github.com/ai4next/superman/internal/config"
)

var (
	configMu sync.RWMutex
	cfg      *config.Config
)

// Config returns the process-wide loaded configuration.
func Config() *config.Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return cfg
}

// LoadConfig loads configuration from disk and stores it process-wide.
func LoadConfig(path string) (*config.Config, error) {
	c, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	SetConfig(c)
	return c, nil
}

// SetConfig replaces the process-wide loaded configuration.
func SetConfig(c *config.Config) {
	configMu.Lock()
	defer configMu.Unlock()
	cfg = c
}
