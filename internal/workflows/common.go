package workflows

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/sashabaranov/go-openai"
)

type ChatSessionWorkflow interface {
	Execute(context.Context) (any, bool, error)
}

type TemplateData struct {
	// Date provides current date context for time-sensitive research
	Date string `json:"date"`

	// Messages contains the conversation history for context
	Messages []openai.ChatCompletionMessage `json:"messages"`

	// ResearchBrief contains the research brief from the user
	ResearchBrief string `json:"research_brief"`

	// RawResearchNote contains the raw research note from the web search tool
	RawResearchNote string `json:"raw_research_note"`

	// CompressedResearchNote contains the compressed research note from the web search tool
	CompressedResearchNotes []string `json:"compressed_research_notes"`
}

func PromptBuilder(templateName, templateStr string, data any) (string, error) {
	// Parse the template
	tmpl, err := template.New(templateName).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template into a buffer
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Return the final prompt string
	return buf.String(), nil
}

func BuildConversationHistory(systemPrompt *string, pastMessages *[]openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	conversationHistory := make([]openai.ChatCompletionMessage, 0, len(*pastMessages)+1)

	if *systemPrompt != "" {
		conversationHistory = append(conversationHistory, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: *systemPrompt,
		})
	}

	return append(conversationHistory, *pastMessages...)
}
