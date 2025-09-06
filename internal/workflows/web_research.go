package workflows

import (
	"context"
	"deep-research/internal/tools"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/instructor-ai/instructor-go/pkg/instructor"
	"github.com/sashabaranov/go-openai"
)

var webSearchPrompt string = `
<ROLE>
You are a research assistant conducting research on the user's input topic.
</ROLE>

<DATE>
For context, today's date is {{ .Date }}.
</DATE>

<TASK>
Your job is to use the tools provided to gather information and resources that directly address the user's research question. 
'Resources' refer to evidence-based materials such as articles, official reports, or studies relevant to the user's topic. 
An 'answer' is considered complete when it is comprehensive, directly addresses the research question, and is supported by at least three distinct, relevant sources, or when further searching yields only information already found.
</TASK>

<AVAILABLE_TOOLS>
You have access to two main tools:
1. **search_tool**: For conducting web searches to gather information
2. **think_tool**: For reflection and strategic planning during research

**CRITICAL: Use think_tool after each search to reflect on results and plan next steps**
</AVAILABLE_TOOLS>

<INSTRUCTIONS>
Think like a human researcher with limited time. Follow these steps:

1. **Read the question carefully** – Determine what specific information the user needs.
2. **Start with broader searches** – Use broad, comprehensive queries first to gather general information.
3. **After each search, pause and assess** – Use think_tool to evaluate if you have enough to answer; identify what’s still missing.
4. **Execute narrower searches as needed** – Use targeted queries to fill specific informational gaps.
5. **Stop when you can answer confidently** – Provide the answer when criteria are met; avoid unnecessary searching.
</INSTRUCTIONS>

<DEFINITIONS>
- **Simple Query**: A question seeking factual, straightforward information on a single aspect or concept.
- **Complex Query**: A question requiring synthesis of multiple pieces of information, addresses multiple components, or explores nuanced or multifaceted topics.
</DEFINITIONS>

<HARD_LIMITS>
**Tool Call Budgets:**
- **Simple queries**: Use 2-3 search_tool calls maximum.
- **Complex queries**: Use up to 5 search_tool calls maximum.
- **Always stop**: After 5 search_tool calls, even if a full answer is not found.

**Stop Immediately When:**
- You can answer the user's question comprehensively, supported by at least three distinct, relevant sources.
- Your last two searches each returned similar or redundant information.
</HARD_LIMITS>

<DECISION CRITERIA>
After each search and reflection (reflection_tool):
- If you have found three or more relevant sources covering the question, or
- If subsequent searches only yield repeated information, or
- If you can directly and comprehensively answer the research question,
Then proceed to answer; otherwise, continue searching within tool call limits.
</DECISION CRITERIA>

<SHOW_YOUR_THINKING>
After each search tool call, use reflection_tool to analyze the results:
- What key information did I find?
- What information is still missing?
- Do I now have enough to fully answer the question?
- Should I perform another search or provide my answer based on current findings?
</SHOW_YOUR_THINKING>
`

var summarizeWebSeachResultPrompt string = `
<ROLE>
You are a summarization agent tasked with condensing the raw content of a webpage into a concise summary that preserves the most important information from the original page.
Your summary will be used by a downstream research agent, so it is essential to retain key details and facts.
</ROLE>

<DATE>
For context, today's date is {{ .Date }}.
</DATE>

<WEBPAGE_CONTENT>
{{ .RawResearchNote }}
</WEBPAGE_CONTENT>

<INSTRUCTIONS>
Summarize the content to be approximately 25–30% of the original length, unless already concise, while allowing the summary to stand alone as a complete source of information.
</INSTRUCTIONS>

<GUIDELINES>
Follow these guidelines:
1. Identify and preserve the main topic or purpose of the webpage.
2. Retain central facts, statistics, and data points.
3. Keep important quotes from credible sources or experts (up to 5 in total).
4. Maintain chronological order for time-sensitive or historical content.
5. Preserve any lists or step-by-step instructions present in the content.
6. Include essential dates, names, and locations.
7. Summarize lengthy explanations without omitting core messages.

For specific types of content, apply these focus areas:
- **News articles:** Emphasize who, what, when, where, why, and how.
- **Scientific content:** Preserve methodology, results, and conclusions.
- **Opinion pieces:** Maintain key arguments and supporting points.
- **Product pages:** Retain key features, specifications, and unique selling points.
</GUIDELINES>

<OUTPUT_FORMAT>
Your output must be valid JSON matching the following schema:
{
   "summary": "<Your summary here, structured with appropriate paragraphs or bullet points as needed>",
   "key_excerpts": "<Important quote or excerpt one, Important quote or excerpt two, ... (up to 5 quotes/excerpts, comma-separated)>"
}
</OUTPUT_FORMAT>

<EXAMPLES>
Example 1 (news article):
{
  "summary": "On July 15, 2023, NASA successfully launched the Artemis II mission from Kennedy Space Center. This marks the first crewed mission to the Moon since Apollo 17 in 1972. The four-person crew, led by Commander Jane Smith, will orbit the Moon for 10 days before returning to Earth. This mission is a crucial step in NASA's plans to establish a permanent human presence on the Moon by 2030.",
  "key_excerpts": "Artemis II represents a new era in space exploration, said NASA Administrator John Doe. The mission will test critical systems for future long-duration stays on the Moon, explained Lead Engineer Sarah Johnson. We're not just going back to the Moon, we're going forward to the Moon, Commander Jane Smith stated during the pre-launch press conference."
}

Example 2 (scientific article):
{
  "summary": "A new study published in Nature Climate Change reveals that global sea levels are rising faster than previously thought. Researchers analyzed satellite data from 1993 to 2022 and found that the rate of sea-level rise has accelerated by 0.08 mm/year². This increase is mainly due to melting ice sheets in Greenland and Antarctica and may result in sea levels rising by up to 2 meters by 2100, threatening coastal communities globally.",
  "key_excerpts": "Our findings indicate a clear acceleration in sea-level rise, which has significant implications for coastal planning and adaptation strategies, lead author Dr. Emily Brown stated. The rate of ice sheet melt in Greenland and Antarctica has tripled since the 1990s, the study reports. Without immediate and substantial reductions in greenhouse gas emissions, we are looking at potentially catastrophic sea-level rise by the end of this century, warned co-author Professor Michael Green."
}
</EXAMPLES>

<REMINDER>
Remember, your goal is to create a summary that can be easily understood and utilized by a downstream research agent while preserving the most critical information from the original webpage.
</REMINDER>
`

type WebResearchWorkflow struct {
	client                  *openai.Client
	structuredOutputClient  *instructor.InstructorOpenAI
	logger                  *slog.Logger
	messages                *[]openai.ChatCompletionMessage
	compressedResearchNotes *[]string
}

type SummarizedResearchOutputSchema struct {
	Summary     string `json:"summary"`
	KeyExcerpts string `json:"key_excerpts"`
}

func NewWebResearch(messages *[]openai.ChatCompletionMessage, compressedResearchNotes *[]string, client *openai.Client, structuredOutputClient *instructor.InstructorOpenAI, logger *slog.Logger) ChatSessionWorkflow {
	return &WebResearchWorkflow{
		client:                  client,
		structuredOutputClient:  structuredOutputClient,
		logger:                  logger,
		messages:                messages,
		compressedResearchNotes: compressedResearchNotes,
	}
}

func (wr *WebResearchWorkflow) Execute(ctx context.Context) (any, bool, error) {
	wr.logger.Debug("Executing web research workflow")

	// Build data for prompt
	data := TemplateData{
		Date:     time.Now().Format("02/01/2006"),
		Messages: *wr.messages,
	}
	prompt, err := PromptBuilder("web_research", webSearchPrompt, data)
	if err != nil {
		return "", false, err
	}

	conversationHistory := BuildConversationHistory(&prompt, wr.messages)
	webResearchTools := tools.BuildTools(
		[]openai.FunctionDefinition{tools.SearchToolDefinition, tools.ReflectionToolDefinition})
	resp, err := wr.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:             openai.GPT5,
		Messages:          conversationHistory,
		Tools:             webResearchTools,
		ParallelToolCalls: false,
	})
	if err != nil {
		return "", false, fmt.Errorf("failed to create chat completion: %w", err)
	}
	msg := resp.Choices[0].Message
	*wr.messages = append(*wr.messages, msg)

	if len(msg.ToolCalls) == 0 {
		return "", false, nil
	}

	for _, toolCall := range msg.ToolCalls {
		if toolCall.Function.Name == "search_tool" {
			results, err := tools.SearchTool{}.Execute([]byte(toolCall.Function.Arguments))
			if err != nil {
				return "", false, fmt.Errorf("failed to execute search tool: %w", err)
			}
			summarizedResults, err := summarizeWebSearchResult(ctx, results, wr.compressedResearchNotes, wr.structuredOutputClient)
			if err != nil {
				return "", false, fmt.Errorf("failed to summarize web search results: %w", err)
			}
			*wr.messages = append(*wr.messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    summarizedResults,
				Name:       toolCall.Function.Name,
				ToolCallID: toolCall.ID,
			})
			wr.logger.Debug("Called search tool", "result", summarizedResults)
		}
		if toolCall.Function.Name == "reflection_tool" {
			result, err := tools.ReflectionTool{}.Execute([]byte(toolCall.Function.Arguments))
			if err != nil {
				return "", false, err
			}
			*wr.messages = append(*wr.messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				Name:       toolCall.Function.Name,
				ToolCallID: toolCall.ID,
			})
			wr.logger.Debug("Reflection tool result", "result", result)
		}
	}

	return "", true, nil
}

func summarizeWebSearchResult(ctx context.Context, results []string, compressedResearchNotes *[]string, client *instructor.InstructorOpenAI) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("no results to summarize")
	}

	// Create channels for work distribution and result collection
	type workItem struct {
		index  int
		result string
	}

	type resultItem struct {
		index   int
		summary string
		err     error
	}

	workChan := make(chan workItem, len(results))
	resultChan := make(chan resultItem, len(results))

	// Limit to 5 concurrent workers
	numWorkers := min(len(results), 5)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		go func() {
			for work := range workChan {
				data := TemplateData{
					RawResearchNote: work.result,
				}
				prompt, err := PromptBuilder("summarize_research", summarizeWebSeachResultPrompt, data)
				if err != nil {
					resultChan <- resultItem{index: work.index, err: fmt.Errorf("failed to build prompt: %w", err)}
					continue
				}

				var summarizedResearchNote SummarizedResearchOutputSchema
				resp, err := client.CreateChatCompletion(
					ctx, openai.ChatCompletionRequest{
						Model: openai.GPT4o,
						Messages: []openai.ChatCompletionMessage{
							{
								Role:    openai.ChatMessageRoleUser,
								Content: prompt,
							},
						},
					}, &summarizedResearchNote)
				if err != nil {
					resultChan <- resultItem{
						index: work.index,
						err:   fmt.Errorf("failed to summarize: %w", err),
					}
				}
				_ = resp
				summary := fmt.Sprintf("<summary>\n%s\n</summary>\n<key_excerpts>\n%s\n</key_excerpts>",
					summarizedResearchNote.Summary, summarizedResearchNote.KeyExcerpts)
				resultChan <- resultItem{index: work.index, summary: summary}
			}
		}()
	}

	go func() {
		defer close(workChan)
		for i, result := range results {
			workChan <- workItem{index: i, result: result}
		}
	}()

	summarizedResearchNotes := make([]string, len(results))
	for i := 0; i < len(results); i++ {
		result := <-resultChan
		if result.err != nil {
			return "", result.err
		}
		summarizedResearchNotes[result.index] = result.summary
	}

	*compressedResearchNotes = append(*compressedResearchNotes, summarizedResearchNotes...)
	return strings.Join(summarizedResearchNotes, "\n"), nil
}
