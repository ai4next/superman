package plugin

import (
	"fmt"
	"log"
	"sort"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	adkplugin "google.golang.org/adk/plugin"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

func CreateLoggerPlugin() (*adkplugin.Plugin, error) {
	return adkplugin.New(adkplugin.Config{
		Name: "logger",
		BeforeRunCallback: func(ic agent.InvocationContext) (*genai.Content, error) {
			log.Printf(
				"[logger] before_run invocation=%s session=%s user=%s app=%s agent=%s started_at=%s",
				ic.InvocationID(),
				invocationSessionID(ic),
				invocationUserID(ic),
				invocationAppName(ic),
				invocationAgentName(ic),
				time.Now().Format(time.RFC3339),
			)
			return nil, nil
		},
		AfterRunCallback: func(ic agent.InvocationContext) {
			log.Printf(
				"[logger] after_run invocation=%s session=%s user=%s app=%s agent=%s completed_at=%s",
				ic.InvocationID(),
				invocationSessionID(ic),
				invocationUserID(ic),
				invocationAppName(ic),
				invocationAgentName(ic),
				time.Now().Format(time.RFC3339),
			)
		},
		BeforeModelCallback: func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
			log.Printf(
				"[logger] before_model invocation=%s session=%s agent=%s model=%s contents=%d text_chars=%d function_calls=%d function_responses=%d inline_parts=%d tools=%d config={temperature=%s top_p=%s max_output_tokens=%d system_chars=%d}",
				ctx.InvocationID(),
				ctx.SessionID(),
				ctx.AgentName(),
				req.Model,
				len(req.Contents),
				contentTextLen(req.Contents),
				countParts(req.Contents, func(p *genai.Part) bool { return p.FunctionCall != nil }),
				countParts(req.Contents, func(p *genai.Part) bool { return p.FunctionResponse != nil }),
				countParts(req.Contents, func(p *genai.Part) bool { return p.InlineData != nil || p.FileData != nil }),
				len(req.Tools),
				formatFloat32Ptr(modelTemperature(req)),
				formatFloat32Ptr(modelTopP(req)),
				modelMaxOutputTokens(req),
				systemInstructionTextLen(req),
			)
			return nil, nil
		},
		AfterModelCallback: func(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
			if err != nil {
				log.Printf(
					"[logger] after_model invocation=%s session=%s agent=%s error=%v",
					ctx.InvocationID(),
					ctx.SessionID(),
					ctx.AgentName(),
					err,
				)
				return nil, nil
			}
			if resp == nil {
				log.Printf(
					"[logger] after_model invocation=%s session=%s agent=%s empty_response=true",
					ctx.InvocationID(),
					ctx.SessionID(),
					ctx.AgentName(),
				)
				return nil, nil
			}
			log.Printf(
				"[logger] after_model invocation=%s session=%s agent=%s model_version=%s text_chars=%d function_calls=%d finish_reason=%s partial=%t turn_complete=%t usage={prompt=%d candidates=%d thoughts=%d tool_use=%d total=%d} error_code=%s error_message=%q",
				ctx.InvocationID(),
				ctx.SessionID(),
				ctx.AgentName(),
				resp.ModelVersion,
				contentTextLen([]*genai.Content{resp.Content}),
				countParts([]*genai.Content{resp.Content}, func(p *genai.Part) bool { return p.FunctionCall != nil }),
				resp.FinishReason,
				resp.Partial,
				resp.TurnComplete,
				usagePromptTokens(resp),
				usageCandidateTokens(resp),
				usageThoughtTokens(resp),
				usageToolUseTokens(resp),
				usageTotalTokens(resp),
				resp.ErrorCode,
				resp.ErrorMessage,
			)
			return nil, nil
		},
		BeforeToolCallback: func(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
			log.Printf(
				"[logger] before_tool invocation=%s session=%s agent=%s function_call_id=%s tool=%s args_keys=%v args_size=%d long_running=%t",
				ctx.InvocationID(),
				ctx.SessionID(),
				ctx.AgentName(),
				ctx.FunctionCallID(),
				t.Name(),
				mapKeys(args),
				len(args),
				t.IsLongRunning(),
			)
			return nil, nil
		},
		AfterToolCallback: func(ctx tool.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
			errText := ""
			if err != nil {
				errText = err.Error()
			}
			log.Printf(
				"[logger] after_tool invocation=%s session=%s agent=%s function_call_id=%s tool=%s args_keys=%v result_keys=%v result_size=%d error=%q",
				ctx.InvocationID(),
				ctx.SessionID(),
				ctx.AgentName(),
				ctx.FunctionCallID(),
				t.Name(),
				mapKeys(args),
				mapKeys(result),
				len(result),
				errText,
			)
			return nil, nil
		},
	})
}

func contentTextLen(contents []*genai.Content) int {
	total := 0
	for _, content := range contents {
		if content == nil {
			continue
		}
		for _, part := range content.Parts {
			if part != nil {
				total += len([]rune(part.Text))
			}
		}
	}
	return total
}

func countParts(contents []*genai.Content, match func(*genai.Part) bool) int {
	count := 0
	for _, content := range contents {
		if content == nil {
			continue
		}
		for _, part := range content.Parts {
			if part != nil && match(part) {
				count++
			}
		}
	}
	return count
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func invocationSessionID(ctx agent.InvocationContext) string {
	if ctx == nil || ctx.Session() == nil {
		return ""
	}
	return ctx.Session().ID()
}

func invocationUserID(ctx agent.InvocationContext) string {
	if ctx == nil || ctx.Session() == nil {
		return ""
	}
	return ctx.Session().UserID()
}

func invocationAppName(ctx agent.InvocationContext) string {
	if ctx == nil || ctx.Session() == nil {
		return ""
	}
	return ctx.Session().AppName()
}

func invocationAgentName(ctx agent.InvocationContext) string {
	if ctx == nil || ctx.Agent() == nil {
		return ""
	}
	return ctx.Agent().Name()
}

func systemInstructionTextLen(req *model.LLMRequest) int {
	if req == nil || req.Config == nil || req.Config.SystemInstruction == nil {
		return 0
	}
	return contentTextLen([]*genai.Content{req.Config.SystemInstruction})
}

func modelTemperature(req *model.LLMRequest) *float32 {
	if req == nil || req.Config == nil {
		return nil
	}
	return req.Config.Temperature
}

func modelTopP(req *model.LLMRequest) *float32 {
	if req == nil || req.Config == nil {
		return nil
	}
	return req.Config.TopP
}

func modelMaxOutputTokens(req *model.LLMRequest) int32 {
	if req == nil || req.Config == nil {
		return 0
	}
	return req.Config.MaxOutputTokens
}

func formatFloat32Ptr(v *float32) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%.3f", *v)
}

func usagePromptTokens(resp *model.LLMResponse) int32 {
	if resp == nil || resp.UsageMetadata == nil {
		return 0
	}
	return resp.UsageMetadata.PromptTokenCount
}

func usageCandidateTokens(resp *model.LLMResponse) int32 {
	if resp == nil || resp.UsageMetadata == nil {
		return 0
	}
	return resp.UsageMetadata.CandidatesTokenCount
}

func usageThoughtTokens(resp *model.LLMResponse) int32 {
	if resp == nil || resp.UsageMetadata == nil {
		return 0
	}
	return resp.UsageMetadata.ThoughtsTokenCount
}

func usageToolUseTokens(resp *model.LLMResponse) int32 {
	if resp == nil || resp.UsageMetadata == nil {
		return 0
	}
	return resp.UsageMetadata.ToolUsePromptTokenCount
}

func usageTotalTokens(resp *model.LLMResponse) int32 {
	if resp == nil || resp.UsageMetadata == nil {
		return 0
	}
	return resp.UsageMetadata.TotalTokenCount
}
