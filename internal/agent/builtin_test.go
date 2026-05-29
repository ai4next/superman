package agent

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/model"
	adksession "google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	supermantool "github.com/ai4next/superman/internal/tool"
)

type dynamicExpertManager struct {
}

func (m *dynamicExpertManager) RunDelegate(context.Context, string, string) (string, error) {
	return "", nil
}

var _ supermantool.DelegateRunner = (*dynamicExpertManager)(nil)

type builtinFakeLLM struct{}

func (builtinFakeLLM) Name() string { return "builtin-fake" }

func (builtinFakeLLM) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{
			Content: genai.NewContentFromText("ok", genai.RoleModel),
		}, nil)
	}
}

func TestDynamicToolsProviderRefreshesDelegateTool(t *testing.T) {
	expertsDir := t.TempDir()
	registry := expert.NewRegistry(expertsDir)
	runner := &dynamicExpertManager{}
	provider := dynamicToolsProvider(BuildConfig{
		Config:            &config.Config{},
		ExpertRegistry:    registry,
		DelegateRunner:    runner,
		EnableExpertTools: true,
	})
	ctx := testCallbackContext{}

	req := modelRequest()
	if err := provider(ctx, req); err != nil {
		t.Fatalf("dynamicToolsProvider returned error: %v", err)
	}
	if _, ok := req.Tools["delegate"]; ok {
		t.Fatalf("delegate tool should not be exposed without experts")
	}

	writeExpert(t, expertsDir, "architect")
	req = modelRequest()
	if err := provider(ctx, req); err != nil {
		t.Fatalf("dynamicToolsProvider returned error: %v", err)
	}
	if _, ok := req.Tools["delegate"]; !ok {
		t.Fatalf("delegate tool should be exposed when experts are available")
	}
	decl := firstDeclaration(req, "delegate")
	if decl == nil || !strings.Contains(decl.Description, "architect") {
		t.Fatalf("delegate declaration missing dynamic expert name: %#v", decl)
	}
}

func writeExpert(t *testing.T, expertsDir, name string) {
	t.Helper()
	dir := filepath.Join(expertsDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	data := []byte("test prompt")
	if err := os.WriteFile(filepath.Join(dir, "soul.md"), data, 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func TestProcessDynamicToolsetsSkipsEmptyToolsets(t *testing.T) {
	req := modelRequest()
	toolsets := []tool.Toolset{
		staticToolset{name: "empty", tools: []tool.Tool{testTool(t, "empty_task")}},
		staticToolset{name: "nonempty", tools: []tool.Tool{testTool(t, "runtime_task")}},
	}

	if err := processDynamicToolsets(testCallbackContext{}, req, toolsets); err != nil {
		t.Fatalf("processDynamicToolsets returned error: %v", err)
	}
	if _, ok := req.Tools["runtime_task"]; !ok {
		t.Fatalf("non-empty toolset tool was not exposed")
	}
	if _, ok := req.Tools["empty_task"]; ok {
		t.Fatalf("toolset without context should not expose tools")
	}
	if len(req.Config.Tools) != 1 || len(req.Config.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("unexpected function declarations: %#v", req.Config.Tools)
	}
}

func TestNewSequentialAgentOrganizesPlanExecuteLoop(t *testing.T) {
	a, err := newSequentialAgent(builtinFakeLLM{}, BuildConfig{Name: "superman", Instruction: "base instruction"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name() != "superman" {
		t.Fatalf("agent name = %q", a.Name())
	}
	subagents := a.SubAgents()
	if len(subagents) != 2 {
		t.Fatalf("subagents = %d", len(subagents))
	}
	if subagents[0].Name() != "superman_planner" {
		t.Fatalf("planner name = %q", subagents[0].Name())
	}
	if subagents[1].Name() != "superman_plan_execute_loop" {
		t.Fatalf("loop name = %q", subagents[1].Name())
	}
	loopSubagents := subagents[1].SubAgents()
	if len(loopSubagents) != 2 {
		t.Fatalf("loop subagents = %d", len(loopSubagents))
	}
	if loopSubagents[0].Name() != "superman_executor" {
		t.Fatalf("executor name = %q", loopSubagents[0].Name())
	}
	if loopSubagents[1].Name() != "superman_replanner" {
		t.Fatalf("replanner name = %q", loopSubagents[1].Name())
	}
}

func TestAgentExecutorBeforeModelInjectsRuntimeContext(t *testing.T) {
	executor := newAgentExecutor(BuildConfig{Instruction: "base instruction"})
	req := modelRequest()
	if _, err := executor.beforeModel(testCallbackContext{}, req); err != nil {
		t.Fatal(err)
	}
	instruction := contentText(req.Config.SystemInstruction)
	if !strings.Contains(instruction, "base instruction") {
		t.Fatalf("instruction = %q", instruction)
	}
	if strings.Contains(instruction, "Agent Planner") {
		t.Fatalf("executor instruction should not include planner prompt: %q", instruction)
	}
}

func TestVisiblePlanPromptAddsProgramPrefix(t *testing.T) {
	got := visiblePlanPrompt("planner body")
	if !strings.HasPrefix(got, "计划:\n") || !strings.Contains(got, "planner body") {
		t.Fatalf("visible plan prompt = %q", got)
	}
}

func modelRequest() *model.LLMRequest {
	return &model.LLMRequest{Config: &genai.GenerateContentConfig{}}
}

func firstDeclaration(req *model.LLMRequest, name string) *genai.FunctionDeclaration {
	for _, t := range req.Config.Tools {
		for _, decl := range t.FunctionDeclarations {
			if decl.Name == name {
				return decl
			}
		}
	}
	return nil
}

func testTool(t *testing.T, name string) tool.Tool {
	t.Helper()
	created, err := functiontool.New(functiontool.Config{Name: name, Description: "test tool"}, func(tool.Context, map[string]any) (map[string]any, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("functiontool.New returned error: %v", err)
	}
	return created
}

type staticToolset struct {
	name  string
	tools []tool.Tool
}

func (s staticToolset) Name() string { return s.name }
func (s staticToolset) Tools(ctx adkagent.ReadonlyContext) ([]tool.Tool, error) {
	return s.tools, nil
}

func (s staticToolset) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	if s.name == "nonempty" {
		req.Config.SystemInstruction = genai.NewContentFromText("runtime toolset context", genai.RoleUser)
	}
	return nil
}

type testCallbackContext struct {
	invocationID string
}

func (testCallbackContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (testCallbackContext) Done() <-chan struct{}       { return nil }
func (testCallbackContext) Err() error                  { return nil }
func (testCallbackContext) Value(any) any               { return nil }
func (testCallbackContext) Agent() adkagent.Agent       { return nil }
func (testCallbackContext) Memory() adkagent.Memory     { return nil }
func (testCallbackContext) Session() adksession.Session { return nil }
func (testCallbackContext) UserContent() *genai.Content { return nil }
func (testCallbackContext) RunConfig() *adkagent.RunConfig {
	return nil
}
func (testCallbackContext) EndInvocation() {}
func (testCallbackContext) Ended() bool    { return false }
func (c testCallbackContext) WithContext(context.Context) adkagent.InvocationContext {
	return c
}
func (c testCallbackContext) InvocationID() string {
	if c.invocationID != "" {
		return c.invocationID
	}
	return "invocation"
}
func (testCallbackContext) AgentName() string { return "superman" }
func (testCallbackContext) UserID() string    { return "user" }
func (testCallbackContext) AppName() string   { return "app" }
func (testCallbackContext) SessionID() string { return "session" }
func (testCallbackContext) Branch() string    { return "" }
func (testCallbackContext) ReadonlyState() adksession.ReadonlyState {
	return testState{}
}
func (testCallbackContext) Artifacts() adkagent.Artifacts { return testArtifacts{} }
func (testCallbackContext) State() adksession.State       { return testState{} }

type testState struct{}

func (testState) Get(string) (any, error) { return nil, adksession.ErrStateKeyNotExist }
func (testState) Set(string, any) error   { return nil }
func (testState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {}
}

type testArtifacts struct{}

func (testArtifacts) Save(context.Context, string, *genai.Part) (*artifact.SaveResponse, error) {
	return nil, nil
}
func (testArtifacts) List(context.Context) (*artifact.ListResponse, error) { return nil, nil }
func (testArtifacts) Load(context.Context, string) (*artifact.LoadResponse, error) {
	return nil, nil
}
func (testArtifacts) LoadVersion(context.Context, string, int) (*artifact.LoadResponse, error) {
	return nil, nil
}
