package plugin

import (
	"log"
	"time"

	"google.golang.org/adk/agent"
	adkplugin "google.golang.org/adk/plugin"
)

func CreateSessionReaperPlugin(maxTurns int, maxAge time.Duration) (*adkplugin.Plugin, error) {
	return adkplugin.New(adkplugin.Config{
		Name: "session_reaper",
		AfterRunCallback: func(ic agent.InvocationContext) {
			sess := ic.Session()
			if sess == nil {
				return
			}
			lastUpdate := sess.LastUpdateTime()
			if time.Since(lastUpdate) > maxAge {
				log.Printf("[session_reaper] session %s expired (age: %s)", sess.ID(), time.Since(lastUpdate))
			}
		},
	})
}
