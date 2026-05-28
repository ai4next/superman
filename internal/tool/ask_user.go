package tool

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type askInput struct {
	Question string `json:"question" jsonschema:"Question for the user"`
}

type askOutput struct {
	Answer string `json:"answer"`
}

func newAskTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input askInput) (askOutput, error) {
		return ask(input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "ask",
		Description: "Ask the user",
	}, handler)
	return t
}

func ask(input askInput) (askOutput, error) {
	fmt.Fprintf(os.Stderr, "\n[Agent asks]: %s\n> ", input.Question)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return askOutput{}, fmt.Errorf("failed to read user input: %w", err)
	}
	return askOutput{
		Answer: strings.TrimSpace(answer),
	}, nil
}
