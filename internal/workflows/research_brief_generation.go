package workflows

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/instructor-ai/instructor-go/pkg/instructor"
	"github.com/sashabaranov/go-openai"
)

var transformUserMessageToResearchBriefPrompt string = `
<ROLE>
You are tasked with transforming the full messages history between the user and yourself into a detailed and concrete research brief to guide a research process.
</ROLE>

<DATE>
For context, today's date is {{ .Date }}.
</DATE>

<INSTRUCTIONS>
Return a single research brief, using the entire conversation history between the user and assistant unless otherwise specified. The research brief will serve as the basis to guide subsequent research steps.
If user preferences or requirements are unclear or conflicting, explicitly identify these areas in the brief and treat them as open for clarification or further investigation.
</INSTRUCTIONS>

<DEFINITIONS>
- Attribute: A specific characteristic or feature relevant to the research target (e.g., color, brand, user rating for products).
- Dimension: A broad aspect or category along which options may differ and should be considered (e.g., price range, product category, usability).
- Preference: A user-stated constraint, requirement, or choice expressing what they want (e.g., "must be under $100," "organic only").
- Scope: The set of topics, areas, or dimensions the research should encompass (may be broader than stated user preferences).
</DEFINITIONS>

<GUIDELINES>
1. Maximize Specificity and Detail
- Include all known user preferences. Explicitly list all key attributes and dimensions stated or implied.
- Ensure that no user detail is omitted.

2. Handle Unstated or Unclear Dimensions
- Where research quality requires attention to additional dimensions not specified by the user, list them as open considerations in the brief; do not assume preferences.
- For example: Instead of assuming a preference for lower price, state, "Consider all price ranges unless otherwise specified by the user."
- Only include dimensions necessary for comprehensive research in the context.

3. Avoid Unwarranted Assumptions
- Never invent user preferences or constraints that were not directly stated in the conversation history.
- If a preference or detail is missing, explicitly note its absence and guide the researcher to treat it as flexible.

4. Separate Research Scope from User Preferences
- Research scope: Broader topics or dimensions to be investigated.
- User preferences: Only those constraints and requirements clearly stated by the user.
- Example: "Research coffee quality factors (bean sourcing, roasting, brewing) for San Francisco shops, with primary focus on taste per user instruction."

5. Use the First Person
- Write the research brief from the user's perspective.

6. Preferred Sources
- If the user specifies sources or types of sources to prioritize, clearly note these in the research brief.
- For product/travel, link directly to official or primary sources (e.g., manufacturer websites, Amazon for reviews) over aggregators or SEO blogs.
- For academic/scientific queries, link to original papers or official journal sources over summaries.
- For people, prefer LinkedIn or personal websites.
- If the brief is in a specific language, prioritize sources in that language.
</GUIDELINES>

<MESSAGES>
[
{{range $index, $message := .Messages}}
{"role": "{{$message.Role}}", "content": "{{$message.Content}}"}
{{end}}
]
</MESSAGES>

<OUTPUT_FORMAT>
Your output must be valid JSON matching the following schema:
{
  "research_brief": "<a single research brief that will be used to guide the research process>"
}
</OUTPUT_FORMAT>
`

type ResearchBriefGenerationWorkflow struct {
	client   *instructor.InstructorOpenAI
	logger   *slog.Logger
	messages *[]openai.ChatCompletionMessage
}

// TODO: add jsonschema details
type ResearchBriefGenerationOutputSchema struct {
	ResearchBrief string `json:"research_brief"`
}

func NewResearchBriefGeneration(messages *[]openai.ChatCompletionMessage, client *instructor.InstructorOpenAI, logger *slog.Logger) ChatSessionWorkflow {
	return &ResearchBriefGenerationWorkflow{
		client:   client,
		logger:   logger,
		messages: messages,
	}
}

// Transform the conversation history into a comprehensive research brief.
// Uses structured output to ensure the brief follows the required format
// and contains all necessary details for effective research.
func (rbg *ResearchBriefGenerationWorkflow) Execute(ctx context.Context) (any, bool, error) {
	rbg.logger.Debug("Executing research brief generation workflow")

	// Build data for prompt
	data := TemplateData{
		Date:     time.Now().Format("02/01/2006"),
		Messages: *rbg.messages,
	}
	prompt, err := PromptBuilder("research_brief_generation", transformUserMessageToResearchBriefPrompt, data)
	if err != nil {
		return "", false, err
	}

	var ResearchBriefGenerationResponse ResearchBriefGenerationOutputSchema
	resp, err := rbg.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT5,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}, &ResearchBriefGenerationResponse)
	_ = resp
	if err != nil {
		return "", false, fmt.Errorf("failed to create chat completion: %w", err)
	}

	// Create message to append based on response
	message := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: ResearchBriefGenerationResponse.ResearchBrief,
	}

	*rbg.messages = append(*rbg.messages, message)

	return ResearchBriefGenerationResponse.ResearchBrief, false, nil
}
