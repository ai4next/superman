package prompt

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed template/superman_system.md
var supermanSystem string

//go:embed template/superman_evolver_system.md
var supermanEvolverSystem string

//go:embed template/expert_evolver_system.md
var expertEvolverSystem string

//go:embed template/meta_evolver_system.md
var metaEvolverSystem string

//go:embed template/evolution_runtime.md
var evolutionRuntimeTemplate string

//go:embed template/agent_planner.md
var agentPlanner string

//go:embed template/agent_replanner.md
var agentReplanner string

func init() {
	supermanSystem = strings.TrimSpace(supermanSystem)
	supermanEvolverSystem = strings.TrimSpace(supermanEvolverSystem)
	expertEvolverSystem = strings.TrimSpace(expertEvolverSystem)
	metaEvolverSystem = strings.TrimSpace(metaEvolverSystem)
	evolutionRuntimeTemplate = strings.TrimSpace(evolutionRuntimeTemplate)
	agentPlanner = strings.TrimSpace(agentPlanner)
	agentReplanner = strings.TrimSpace(agentReplanner)
}

func SupermanSystem() string {
	return supermanSystem
}

func SupermanEvolverSystem() string {
	return supermanEvolverSystem
}

func ExpertEvolverSystem() string {
	return expertEvolverSystem
}

func MetaEvolverSystem() string {
	return metaEvolverSystem
}

func AgentPlanner() string {
	return agentPlanner
}

func AgentReplanner() string {
	return agentReplanner
}

func EvolutionRuntime(data any) (string, error) {
	tmpl, err := template.New("evolution_runtime").Parse(evolutionRuntimeTemplate)
	if err != nil {
		return "", fmt.Errorf("parse evolution runtime prompt: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute evolution runtime prompt: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}
