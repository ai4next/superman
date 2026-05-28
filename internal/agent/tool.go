package agent

import (
	"context"
	"fmt"

	adkagent "google.golang.org/adk/agent"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	adksession "google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"

	supermantool "github.com/ai4next/superman/internal/tool"
)

type requestProcessor interface {
	ProcessRequest(adktool.Context, *model.LLMRequest) error
}

func dynamicToolsProvider(build BuildConfig) func(adkagent.CallbackContext, *model.LLMRequest) error {
	expertRegistry := build.ExpertRegistry
	delegateRunner := build.DelegateRunner
	if !build.EnableExpertTools {
		expertRegistry = nil
		delegateRunner = nil
	}
	deps := supermantool.Dependencies{
		Config:         build.Config,
		ExpertManager:  expertRegistry,
		DelegateRunner: delegateRunner,
		ExpertTools:    build.EnableExpertTools,
	}

	return func(ctx adkagent.CallbackContext, req *model.LLMRequest) error {
		if expertRegistry != nil {
			if err := expertRegistry.LoadFromDisk(); err != nil {
				return fmt.Errorf("refresh expert registry: %w", err)
			}
		}
		toolCtx := callbackToolContext{CallbackContext: ctx}
		for _, t := range supermantool.RegisterAll(deps) {
			processor, ok := t.(requestProcessor)
			if !ok {
				return fmt.Errorf("tool %q does not implement request processing", t.Name())
			}
			if err := processor.ProcessRequest(toolCtx, req); err != nil {
				return err
			}
		}

		return processDynamicToolsets(ctx, req, buildToolsets(context.Background(), build.Config))
	}
}

func processDynamicToolsets(ctx adkagent.CallbackContext, req *model.LLMRequest, toolsets []adktool.Toolset) error {
	toolCtx := callbackToolContext{CallbackContext: ctx}
	for _, toolset := range toolsets {
		includeToolset, err := processToolsetContext(toolCtx, req, toolset)
		if err != nil {
			return err
		}
		if !includeToolset {
			continue
		}

		tools, err := toolset.Tools(ctx)
		if err != nil {
			return fmt.Errorf("failed to extract tools from the tool set %q: %w", toolset.Name(), err)
		}
		for _, t := range tools {
			processor, ok := t.(requestProcessor)
			if !ok {
				return fmt.Errorf("tool %q from tool set %q does not implement request processing", t.Name(), toolset.Name())
			}
			if err := processor.ProcessRequest(toolCtx, req); err != nil {
				return err
			}
		}
	}
	return nil
}

func processToolsetContext(ctx adktool.Context, req *model.LLMRequest, toolset adktool.Toolset) (bool, error) {
	processor, ok := toolset.(requestProcessor)
	if !ok {
		tools, err := toolset.Tools(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to extract tools from the tool set %q: %w", toolset.Name(), err)
		}
		return len(tools) > 0, nil
	}

	before := requestContextSnapshot(req)
	if err := processor.ProcessRequest(ctx, req); err != nil {
		return false, fmt.Errorf("process request by toolset %q: %w", toolset.Name(), err)
	}
	return requestContextChanged(req, before), nil
}

type requestContextState struct {
	systemParts int
	systemText  string
	tools       int
	functions   int
}

func requestContextSnapshot(req *model.LLMRequest) requestContextState {
	state := requestContextState{}
	if req == nil || req.Config == nil {
		return state
	}
	if req.Config.SystemInstruction != nil {
		state.systemParts = len(req.Config.SystemInstruction.Parts)
		state.systemText = contentText(req.Config.SystemInstruction)
	}
	state.tools = len(req.Config.Tools)
	for _, t := range req.Config.Tools {
		state.functions += len(t.FunctionDeclarations)
	}
	return state
}

func requestContextChanged(req *model.LLMRequest, before requestContextState) bool {
	after := requestContextSnapshot(req)
	return after.systemParts != before.systemParts ||
		after.systemText != before.systemText ||
		after.tools != before.tools ||
		after.functions != before.functions
}

func contentText(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var out string
	for _, part := range content.Parts {
		if part != nil {
			out += part.Text
		}
	}
	return out
}

type callbackToolContext struct {
	adkagent.CallbackContext
}

func (c callbackToolContext) FunctionCallID() string {
	return ""
}

func (c callbackToolContext) Actions() *adksession.EventActions {
	return &adksession.EventActions{}
}

func (c callbackToolContext) SearchMemory(ctx context.Context, query string) (*adkmemory.SearchResponse, error) {
	return nil, fmt.Errorf("memory search is not available while preparing tool declarations")
}

func (c callbackToolContext) ToolConfirmation() *toolconfirmation.ToolConfirmation {
	return nil
}

func (c callbackToolContext) RequestConfirmation(hint string, payload any) error {
	return fmt.Errorf("tool confirmation is not available while preparing tool declarations")
}
