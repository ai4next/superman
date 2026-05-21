package tools

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type askUserInput struct {
	Question string `json:"question" jsonschema:"The question to ask the user"`
}

type askUserOutput struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func newAskUserTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input askUserInput) (askUserOutput, error) {
		return askUser(input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "ask_user",
		Description: "Interrupt execution to ask the user a question and wait for their response",
	}, handler)
	return t
}

func askUser(input askUserInput) (askUserOutput, error) {
	fmt.Fprintf(os.Stderr, "\n[Agent asks]: %s\n> ", input.Question)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return askUserOutput{}, fmt.Errorf("failed to read user input: %w", err)
	}
	return askUserOutput{
		Question: input.Question,
		Answer:   strings.TrimSpace(answer),
	}, nil
}