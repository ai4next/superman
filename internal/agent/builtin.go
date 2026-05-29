package agent

import (
	"fmt"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/prompt"
)

type agentExecutor struct {
	instructionBuilder  func(adkagent.CallbackContext, *model.LLMRequest) (string, error)
	contentsBuilder     func(adkagent.CallbackContext, *model.LLMRequest) ([]*genai.Content, error)
	dynamicToolsBuilder func(adkagent.CallbackContext, *model.LLMRequest) error
	planKey             string
}

func newAgentExecutor(build BuildConfig) agentExecutor {
	return agentExecutor{
		instructionBuilder:  instructionProvider(build),
		contentsBuilder:     contentsProvider(build),
		dynamicToolsBuilder: dynamicToolsProvider(build),
		planKey:             build.Name + "_plan",
	}
}

func (e agentExecutor) beforeModel(ctx adkagent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	if req == nil {
		return nil, nil
	}
	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}
	instruction, err := e.instructionBuilder(ctx, req)
	if err != nil {
		return nil, err
	}
	contents, err := e.contentsBuilder(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(contents) > 0 {
		req.Contents = append(contents, req.Contents...)
	}
	instruction = appendPromptSection(instruction, executorPlanStepPrompt(ctx, e.planKey))
	setSystemInstruction(req, instruction)
	if err := e.dynamicToolsBuilder(ctx, req); err != nil {
		return nil, err
	}
	return nil, nil
}

func executorPlanStepPrompt(ctx adkagent.CallbackContext, planKey string) string {
	plan := stateString(ctx.ReadonlyState(), planKey)
	if strings.TrimSpace(plan) == "" {
		return "Execute the first concrete step needed for the user's request. Do not attempt unrelated steps."
	}
	return fmt.Sprintf(`## Current Plan
%s

Execute only the first unfinished step in the current plan. Report the result of that step clearly.`, plan)
}

func setSystemInstruction(req *model.LLMRequest, instruction string) {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		req.Config.SystemInstruction = nil
		return
	}
	req.Config.SystemInstruction = genai.NewContentFromText(instruction, genai.RoleUser)
}

func appendPromptSection(instruction, section string) string {
	instruction = strings.TrimSpace(instruction)
	section = strings.TrimSpace(section)
	if instruction == "" {
		return section
	}
	if section == "" {
		return instruction
	}
	return instruction + "\n\n" + section
}

func visiblePlanPrompt(plannerPrompt string) string {
	plannerPrompt = strings.TrimSpace(plannerPrompt)
	if plannerPrompt == "" {
		return ""
	}
	return "计划:\n" + plannerPrompt
}

func replannerInstructionProvider(build BuildConfig) llmagent.InstructionProvider {
	return func(ctx adkagent.ReadonlyContext) (string, error) {
		base := visibleReplanPrompt(prompt.AgentReplanner())
		plan := stateString(ctx.ReadonlyState(), build.Name+"_plan")
		result := stateString(ctx.ReadonlyState(), build.Name+"_executor_result")
		if plan == "" && result == "" {
			return base, nil
		}
		var b strings.Builder
		b.WriteString(base)
		if plan != "" {
			b.WriteString("\n\n## Current Plan\n")
			b.WriteString(plan)
		}
		if result != "" {
			b.WriteString("\n\n## Most Recent Executor Result\n")
			b.WriteString(result)
		}
		return strings.TrimSpace(b.String()), nil
	}
}

func visibleReplanPrompt(replannerPrompt string) string {
	replannerPrompt = strings.TrimSpace(replannerPrompt)
	if replannerPrompt == "" {
		return ""
	}
	return "复盘:\n" + replannerPrompt
}

func stateString(state adksession.ReadonlyState, key string) string {
	if state == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	value, err := state.Get(key)
	if err != nil || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
