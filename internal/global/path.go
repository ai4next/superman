package global

import "path/filepath"

func SkillsDir() string {
	return filepath.Join(Config().Workspace, "skills")
}

func HooksDir() string {
	return filepath.Join(Config().Workspace, "hooks")
}

func MemoryDir() string {
	return filepath.Join(Config().Workspace, "memory")
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
	return filepath.Join(Config().Workspace, "state.db")
}

func SessionsDir() string {
	return filepath.Join(Config().Workspace, "sessions")
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
	return filepath.Join(Config().Workspace, "evolution")
}

func EvolutionMemoryDir() string {
	return filepath.Join(EvolutionDir(), "memory")
}

func ExpertsDir() string {
	return filepath.Join(Config().Workspace, "experts")
}

func ExpertDir(name string) string {
	return filepath.Join(ExpertsDir(), name)
}

func ExpertMemoryDir(name string) string {
	return filepath.Join(ExpertDir(name), "memory")
}
