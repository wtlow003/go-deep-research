package workflows

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructor-ai/instructor-go/pkg/instructor"
	"github.com/sashabaranov/go-openai"
)

var clarifyWithUserPrompt string = `
<ROLE>
You are tasked with analyzing a user's message history to determine whether further clarification is required before beginning research.
</ROLE>

<DATE>
For context, today's date is {{ .Date }}.
</DATE>

<INSTRUCTIONS>
- Review the provided message history.
- Only ask clarifying questions when essential details are missing (e.g., unclear scope, missing parameters).
- Avoid repeating questions already asked in the message history unless new ambiguity arises.
- Request clarification for:
  * Undefined acronyms or abbreviations
  * Terms not found in standard dictionaries or field glossaries
  * Any references that could affect understanding user intent
- Do not ask about information already provided.
- When asking questions:
  * Be concise and well-structured
  * Use bullet points or numbered lists when appropriate
  * Stay focused and avoid redundancy

<MINIMUM_INFORMATION>
Proceed with research only when scope, context, parameters, and terminology are clear and unambiguous. Otherwise, seek clarification.
</MINIMUM_INFORMATION>

<MESSAGES>
[
{{range $index, $message := .Messages}}
{"role": "{{$message.Role}}", "content": "{{$message.Content}}"}
{{end}}
]
</MESSAGES>

<OUTPUT_FORMAT>
Respond with JSON following this schema:
{
    "need_clarification": boolean,
    "question": "string",
    "verification": "string"
}

When clarification needed:
- Set need_clarification: true
- Provide clear questions in question field
- Leave verification empty

When no clarification needed:
- Set need_clarification: false
- Leave question empty
- Provide verification message that:
  * Confirms sufficient information received
  * Summarizes understanding
  * States research will begin
</OUTPUT_FORMAT>

<EXAMPLES>
Clarification needed:
{
    "need_clarification": true,
    "question": "Please clarify the meaning of [term] and specify [missing detail]",
    "verification": ""
}

No clarification needed:
{
    "need_clarification": false,
    "question": "",
    "verification": "Information complete for [scope]. Will research [specific topic/parameters] as requested."
}
</EXAMPLES>
`

type ClarifyWithUserWorkflow struct {
	client   *instructor.InstructorOpenAI
	logger   *slog.Logger
	messages *[]openai.ChatCompletionMessage
}

type ClarifyWithUserOutputSchema struct {
	NeedClarification bool   `json:"need_clarification" jsonschema:"title=need clarification,description=whether the user needs to be asked a clarification question,example=false,example=true"`
	Question          string `json:"question" jsonschema:"title=question,description=the question to ask the user,example=Please clarify the meaning of [term] and specify [missing detail]"`
	Verification      string `json:"verification" jsonschema:"title=verification,description=the verification message to confirm sufficient information received,example=Information complete for [scope]. Will research [specific topic/parameters] as requested."`
}

func NewClarifyWithUser(messages *[]openai.ChatCompletionMessage, client *instructor.InstructorOpenAI, logger *slog.Logger) ChatSessionWorkflow {
	return &ClarifyWithUserWorkflow{
		client:   client,
		logger:   logger,
		messages: messages,
	}
}

// Determine if the user's request contains sufficient information to proceed with research.
// Uses structured output to make deterministic decisions and avoid hallucination.
// Routes to either research brief generation or ends with a clarification question.
func (cwu *ClarifyWithUserWorkflow) Execute(ctx context.Context) (any, bool, error) {
	cwu.logger.Debug("Executing clarify with user workflow")

	// Build data for prompt
	data := TemplateData{
		Date:     time.Now().Format("02/01/2006"),
		Messages: *cwu.messages,
	}
	prompt, err := PromptBuilder("clarify_with_user", clarifyWithUserPrompt, data)
	if err != nil {
		return "", false, err
	}

	var ClarifyWithUserResponse ClarifyWithUserOutputSchema
	resp, err := cwu.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT5,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}, &ClarifyWithUserResponse)
	_ = resp
	if err != nil {
		return "", false, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if !ClarifyWithUserResponse.NeedClarification {
		*cwu.messages = append(*cwu.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: ClarifyWithUserResponse.Verification,
		})
		return ClarifyWithUserResponse.Verification, false, nil
	}

	*cwu.messages = append(*cwu.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: ClarifyWithUserResponse.Question,
	})
	return ClarifyWithUserResponse.Question, true, nil
}
