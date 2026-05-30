package agent

import (
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/model"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/exitlooptool"

	"github.com/ai4next/superman/internal/prompt"
)

const defaultPlanExecuteMaxIterations = 6

func newSequentialAgent(llm model.LLM, build BuildConfig) (adkagent.Agent, error) {
	planner, err := newPlannerAgent(llm, build)
	if err != nil {
		return nil, err
	}
	loop, err := newPlanExecuteLoop(llm, build)
	if err != nil {
		return nil, err
	}
	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: adkagent.Config{
			Name:      build.Name,
			SubAgents: []adkagent.Agent{planner, loop},
		},
	})
}

func newPlanExecuteLoop(llm model.LLM, build BuildConfig) (adkagent.Agent, error) {
	executor, err := newExecutorAgent(llm, build)
	if err != nil {
		return nil, err
	}
	replanner, err := newReplannerAgent(llm, build)
	if err != nil {
		return nil, err
	}
	return loopagent.New(loopagent.Config{
		AgentConfig: adkagent.Config{
			Name:      build.Name + "_plan_execute_loop",
			SubAgents: []adkagent.Agent{executor, replanner},
		},
		MaxIterations: defaultPlanExecuteMaxIterations,
	})
}

func newPlannerAgent(llm model.LLM, build BuildConfig) (adkagent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:                     build.Name + "_planner",
		Model:                    llm,
		Instruction:              visiblePlanPrompt(prompt.AgentPlanner()),
		IncludeContents:          llmagent.IncludeContentsDefault,
		OutputKey:                build.Name + "_plan",
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
	})
}

func newExecutorAgent(llm model.LLM, build BuildConfig) (adkagent.Agent, error) {
	executor := newAgentExecutor(build)
	return llmagent.New(llmagent.Config{
		Name:  build.Name + "_executor",
		Model: llm,
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{
			executor.beforeModel,
		},
		OutputKey:                build.Name + "_executor_result",
		IncludeContents:          llmagent.IncludeContentsDefault,
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
	})
}

func newReplannerAgent(llm model.LLM, build BuildConfig) (adkagent.Agent, error) {
	exitLoop, err := exitlooptool.New()
	if err != nil {
		return nil, err
	}
	return llmagent.New(llmagent.Config{
		Name:                     build.Name + "_replanner",
		Model:                    llm,
		InstructionProvider:      replannerInstructionProvider(build),
		Tools:                    []adktool.Tool{exitLoop},
		OutputKey:                build.Name + "_plan",
		IncludeContents:          llmagent.IncludeContentsDefault,
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
	})
}
