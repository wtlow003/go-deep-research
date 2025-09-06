package tools

import (
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type ReflectionTool struct {
	Reflection string `json:"reflection" jsonschema:"title=reflection,description=a structured tool to enhance reflection on research progress and informed decision-making,required"`
}

var ReflectionToolDefinition = openai.FunctionDefinition{
	Name:        "reflection_tool",
	Description: "Reflect on the conversation and provide insights and determine if the research is complete",
	Parameters:  GenerateToolSchema[ReflectionTool](),
}

func (r ReflectionTool) Execute(input json.RawMessage) (string, error) {
	var params ReflectionTool
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	return fmt.Sprintf("Reflection recorded: %s", params.Reflection), nil
}
