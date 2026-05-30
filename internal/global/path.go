package global

import "path/filepath"

func SkillsDir() string {
	return filepath.Join(Config().Workspace, "skills")
}

func HooksDir() string {
	return filepath.Join(Config().Workspace, "hooks")
}

func MemoryDir() string {
	return AgentMemoryDir("superman")
}

func GlobalDBPath() string {
	return filepath.Join(StateRootDir(), "state.db")
}

func MemoryRootDir() string {
	return filepath.Join(Config().Workspace, "memory")
}

func AgentMemoryDir(name string) string {
	return filepath.Join(MemoryRootDir(), name)
}

func L2Dir() string {
	return filepath.Join(MemoryDir(), "l2")
}

func MemoryL1Path(memoryDir string) string {
	return filepath.Join(memoryDir, "l1.toml")
}

func MemoryL2Dir(memoryDir string) string {
	return filepath.Join(memoryDir, "l2")
}

func StateDBPath() string {
	return AgentStateDBPath("superman")
}

func StateRootDir() string {
	return filepath.Join(Config().Workspace, "state")
}

func AgentStateDir(name string) string {
	return filepath.Join(StateRootDir(), name)
}

func AgentStateDBPath(name string) string {
	return filepath.Join(AgentStateDir(name), "state.db")
}

func SessionsDir() string {
	return AgentSessionsDir("superman")
}

func AgentSessionsDir(name string) string {
	return filepath.Join(Config().Workspace, "session", name)
}

func SessionLogPath(sessionID string) string {
	return filepath.Join(SessionsDir(), sessionID+".log")
}

func SessionSnapshotsDir() string {
	return filepath.Join(SessionsDir(), "snapshots")
}

func SessionSnapshotPath(hash string) string {
	return filepath.Join(SessionSnapshotsDir(), hash[:2], hash)
}

func RuntimeDir() string {
	return filepath.Join(Config().Workspace, "runtime")
}

// RuntimeEventsPath is kept for compatibility. Runtime events are stored in the bus audit log.
func RuntimeEventsPath() string {
	return BusEventsPath()
}

func BusDir() string {
	return filepath.Join(Config().Workspace, "bus")
}

func BusDBPath() string {
	if Config().Bus.Path != "" {
		return Config().Bus.Path
	}
	return ""
}

func BusEventsPath() string {
	if Config().Bus.AuditLog != "" {
		return Config().Bus.AuditLog
	}
	return filepath.Join(BusDir(), "events.jsonl")
}

func OrchestratorDir() string {
	return filepath.Join(Config().Workspace, "orchestrator")
}

func PlansDir() string {
	return filepath.Join(OrchestratorDir(), "plans")
}

func EvolutionDir() string {
	return AgentStateDir("evolution")
}

func EvolutionMemoryDir() string {
	return AgentMemoryDir("evolution")
}

func ExpertsDir() string {
	return StateRootDir()
}

func ExpertDir(name string) string {
	return AgentStateDir(name)
}

func ExpertMemoryDir(name string) string {
	return AgentMemoryDir(name)
}
