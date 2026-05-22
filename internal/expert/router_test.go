package expert

import "testing"

func TestRouterDelegatesHighConfidenceMatch(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	if _, err := r.Create(Spec{
		Name:           "go-reviewer",
		Summary:        "Reviews Go code and fixes tests",
		Description:    "Handles Go test failures",
		TriggerPattern: "go test review",
		Status:         StatusActive,
		Confidence:     0.9,
		Capabilities:   []string{"go tests", "code review"},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	router := NewRouter(r)
	candidates := router.Route(ExpertTask{Task: "please review Go code and fix the failing test"})
	if len(candidates) == 0 {
		t.Fatal("expected route candidate")
	}
	if candidates[0].Expert.Name != "go-reviewer" {
		t.Fatalf("top candidate = %s", candidates[0].Expert.Name)
	}
	if candidates[0].Decision != RouteDelegate {
		t.Fatalf("decision = %s, want delegate", candidates[0].Decision)
	}
}

func TestRouterSkipsBelowPolicyThreshold(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	if _, err := r.Create(Spec{
		Name:           "python-reviewer",
		Summary:        "Reviews Python code",
		TriggerPattern: "python",
		Status:         StatusActive,
		Confidence:     0.5,
		RoutingPolicy:  RoutingPolicy{MinConfidence: 0.95},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	candidates := NewRouter(r).Route(ExpertTask{Task: "python review"})
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %#v", candidates)
	}
}
