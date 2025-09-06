package tools

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	"github.com/sashabaranov/go-openai"
)

type ToolFunc func(input json.RawMessage) (string, error)

func GenerateToolSchema[T any]() *jsonschema.Schema {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)

	return schema
}

func BuildTools(tools []openai.FunctionDefinition) []openai.Tool {
	openaiTools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		openaiTools[i] = openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: &tool,
		}
	}
	return openaiTools
}
