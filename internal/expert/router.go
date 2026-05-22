package expert

import (
	"sort"
	"strings"
	"time"
)

type RouteDecision string

const (
	RouteNone     RouteDecision = "none"
	RouteConsult  RouteDecision = "consult"
	RouteDelegate RouteDecision = "delegate"
)

type RouteCandidate struct {
	Expert     *Spec
	Score      float64
	Decision   RouteDecision
	Reason     string
	Confidence float64
}

type Router struct {
	registry *Registry
}

func NewRouter(registry *Registry) *Router {
	return &Router{registry: registry}
}

func (r *Router) Route(task ExpertTask) []RouteCandidate {
	if r == nil || r.registry == nil || strings.TrimSpace(task.Task) == "" {
		return nil
	}
	specs := r.registry.List()
	candidates := make([]RouteCandidate, 0, len(specs))
	for _, spec := range specs {
		score := routeScore(spec, task)
		minConfidence := spec.RoutingPolicy.MinConfidence
		if minConfidence == 0 {
			minConfidence = 0.35
		}
		if score < minConfidence {
			continue
		}
		decision := RouteConsult
		if score >= 0.5 && supportsMode(spec, ModeDelegate) {
			decision = RouteDelegate
		}
		candidates = append(candidates, RouteCandidate{
			Expert:     spec,
			Score:      score,
			Decision:   decision,
			Reason:     "matched expert routing profile",
			Confidence: score,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Expert.UpdatedAt.After(candidates[j].Expert.UpdatedAt)
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func routeScore(spec *Spec, task ExpertTask) float64 {
	taskTokens := tokenSet(strings.Join([]string{task.Task, task.Goal, strings.Join(task.Constraints, " ")}, " "))
	if len(taskTokens) == 0 {
		return 0
	}
	profile := strings.Join([]string{
		spec.Name,
		spec.Summary,
		spec.Description,
		spec.Domain,
		spec.TriggerPattern,
		strings.Join(spec.Capabilities, " "),
	}, " ")
	overlap := jaccard(taskTokens, tokenSet(profile))
	score := overlap*0.65 + spec.Confidence*0.25 + statusBoost(spec.Status)
	if spec.Metrics.TotalCalls > 0 {
		score += spec.Metrics.SuccessRate * 0.1
	}
	if spec.Status == StatusArchived {
		return 0
	}
	return clamp(score, 0, 1)
}

func statusBoost(status Status) float64 {
	switch status {
	case StatusMature:
		return 0.12
	case StatusActive:
		return 0.08
	case StatusDraft:
		return 0.02
	default:
		return 0
	}
}

func supportsMode(spec *Spec, mode CallMode) bool {
	if len(spec.RoutingPolicy.Modes) == 0 {
		return true
	}
	for _, m := range spec.RoutingPolicy.Modes {
		if m == mode {
			return true
		}
	}
	return false
}

func NewTask(task string, tools []string) ExpertTask {
	return ExpertTask{
		Task:           strings.TrimSpace(task),
		AvailableTools: append([]string(nil), tools...),
		ExpectedOutput: "Return an ExpertResult JSON object with summary, findings, risks, next_steps, confidence, and success.",
	}
}

func nowRecord(action EvolutionAction, expert, reason string, evidence []string) EvolutionRecord {
	return EvolutionRecord{
		Timestamp: time.Now(),
		Action:    action,
		Expert:    expert,
		Reason:    reason,
		Evidence:  append([]string(nil), evidence...),
	}
}
