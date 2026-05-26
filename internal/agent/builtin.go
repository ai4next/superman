package agent

import (
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	adkplugin "google.golang.org/adk/plugin"
	"google.golang.org/genai"
)

func NewBuiltin(build BuildConfig) (*adkplugin.Plugin, error) {
	instructionBuilder := instructionProvider(build)
	contentsBuilder := contentsProvider(build)
	return adkplugin.New(adkplugin.Config{
		Name: "builtin",
		BeforeModelCallback: func(ctx adkagent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
			if req == nil {
				return nil, nil
			}
			if req.Config == nil {
				req.Config = &genai.GenerateContentConfig{}
			}
			instruction, err := instructionBuilder(ctx, req)
			if err != nil {
				return nil, err
			}
			if instruction == "" {
				req.Config.SystemInstruction = nil
			} else {
				req.Config.SystemInstruction = genai.NewContentFromText(instruction, genai.RoleUser)
			}
			contents, err := contentsBuilder(ctx, req)
			if err != nil {
				return nil, err
			}
			if len(contents) > 0 {
				req.Contents = append(contents, req.Contents...)
			}
			return nil, nil
		},
	})
}
