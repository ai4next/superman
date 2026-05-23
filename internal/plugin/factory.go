package plugin

import (
	"time"

	adkplugin "google.golang.org/adk/plugin"
)

// Create creates a plugin by config name with sensible defaults.
func Create(name string) (*adkplugin.Plugin, error) {
	switch name {
	case "logger":
		return CreateLoggerPlugin()
	case "session_reaper":
		return CreateSessionReaperPlugin(100, 2*time.Hour)
	default:
		return nil, nil
	}
}
