package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/adk/session"
)

type Manager struct {
	service     session.Service
	historyPath string
	maxTurns    int
	mu          sync.Mutex
}

func New(service session.Service, historyPath string, maxTurns int) *Manager {
	return &Manager{
		service:     service,
		historyPath: historyPath,
		maxTurns:    maxTurns,
	}
}

func (m *Manager) Service() session.Service {
	return m.service
}

type TurnInfo struct {
	Turn      int       `json:"turn"`
	Timestamp time.Time `json:"timestamp"`
	UserMsg   string    `json:"user_message"`
	AgentMsg  string    `json:"agent_response"`
	ToolCalls int       `json:"tool_calls"`
}

func (m *Manager) SaveTurn(sessionID string, turn TurnInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.historyPath, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	line := fmt.Sprintf(`{"turn":%d,"timestamp":"%s","user_message":%q,"agent_response":%q,"tool_calls":%d}`+"\n",
		turn.Turn, turn.Timestamp.Format(time.RFC3339), turn.UserMsg, turn.AgentMsg, turn.ToolCalls)
	_, err = f.WriteString(line)
	return err
}

func (m *Manager) ShouldReap(turn int) bool {
	return turn >= m.maxTurns
}