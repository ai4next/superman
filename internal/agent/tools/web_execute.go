package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type webExecuteInput struct {
	Script string `json:"script" jsonschema:"JavaScript code to execute in the browser"`
}

type webExecuteOutput struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func newWebExecuteTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input webExecuteInput) (webExecuteOutput, error) {
		return webExecuteOutput{
			Error: "Browser automation is not yet available. This feature requires a ChromeDP driver which will be added in a future version.",
		}, fmt.Errorf("web_execute not available: browser driver not configured")
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "web_execute",
		Description: "Execute JavaScript in a browser (requires ChromeDP driver - not yet available)",
	}, handler)
	return t
}
