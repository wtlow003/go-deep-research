package llm

import (
	"deep-research/internal/config"

	"github.com/instructor-ai/instructor-go/pkg/instructor"
	openai "github.com/sashabaranov/go-openai"
)

func InitializeClients(cfg *config.Config) (*openai.Client, *instructor.InstructorOpenAI, error) {

	client := openai.NewClient(cfg.OpenAIAPIKey)
	structuredOutputClient := instructor.FromOpenAI(
		client,
		instructor.WithMode(instructor.ModeJSONSchema),
		instructor.WithMaxRetries(3),
	)

	return client, structuredOutputClient, nil
}
