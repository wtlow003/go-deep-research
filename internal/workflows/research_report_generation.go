package workflows

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/instructor-ai/instructor-go/pkg/instructor"
	"github.com/sashabaranov/go-openai"
)

var writeResearchReportPrompt string = `
<ROLE>
You are tasked with writing a professional research report based on a provided research brief and findings, using only the input materials.
</ROLE>

<DATE>
For context, today's date is {{ .Date }}.
</DATE>

<RESEARCH_BRIEF>
{{ .ResearchBrief }}
</RESEARCH_BRIEF>

<FINDINGS>
Here are the findings from the research that you conducted:
[
{{range $index, $compressedResearchNote := .CompressedResearchNotes}}
{{$compressedResearchNote}}
{{end}}
]
</FINDINGS>

<INSTRUCTIONS>
Based only on the findings provided in <FINDINGS>, create a comprehensive, well-structured report addressing the research brief <RESEARCH_BRIEF>. Do not use external knowledge or information.
</INSTRUCTIONS>

<GUIDELINES>
The report must:
1. Be well-organized with appropriate headings (# for the title, ## for sections, ### for subsections) in Markdown.
2. Include specific facts and insights only from the provided research findings.
3. Reference relevant sources in [Title](URL) format.
4. Provide a balanced and thorough analysis, including all relevant information from the findings.
5. End with a "### Sources" section listing all sources referenced in the text.
6. Assign each unique cited URL a single citation number in sequential order (1, 2, 3, ...), ignoring any numbers assigned in the source input. Use these numbers for in-text citations and in the "Sources" list.
7. Write in simple, clear language and use paragraphs by default; bullet points are permitted when appropriate.
8. Use Markdown formatting for structure and clarity.
9. Do not refer to yourself, the writer, or the process of writing the report. Provide the report as if it were standalone.
10. Ensure each section is sufficiently detailed, using the research findings as completely as possible.

Section structure may vary depending on the nature of the brief. Some example structures:
- For comparisons: introduction, overview of item A, overview of item B, comparison, conclusion
- For lists: itemized sections or a consolidated list
- For summaries: overview, relevant concepts, conclusion
- For single-focus questions: one section with a comprehensive answer
- Choose the most logical and useful structure for the brief

Each report section:
- Use '##' for section titles (Markdown format)
- Do not include commentary on the writing process
- Length should be appropriate to the depth available in the provided findings
- Follow Markdown best practices for lists, headings, and links
</GUIDELINES>

</CITATION_RULES>
- Assign each unique URL a single citation number in your text
- End with ### Sources that lists each source with corresponding numbers
- IMPORTANT: Number sources sequentially without gaps (1,2,3,4...) in the final list regardless of which sources you choose
- Each source should be a separate line item in a list, so that in markdown it is rendered as a list.
- Example format:
  [1] Source Title: URL
  [2] Source Title: URL
- Citations are extremely important. Make sure to include these, and pay a lot of attention to getting these right. Users will often use these citations to look into more information.
</CITATION_RULES>

<OUTPUT_FORMAT>
Your output must be valid JSON matching the following schema:
{
  "report": "<a single research report that is based on the research process and findings>"
}
</OUTPUT_FORMAT>
`

type ResearchReportGeneration struct {
	client                  *instructor.InstructorOpenAI
	logger                  *slog.Logger
	researchBrief           *string
	compressedResearchNotes *[]string
}

type ResearchReportGenerationOutputSchema struct {
	Report string `json:"report"`
}

func NewResearchReportGeneration(researchBrief *string, compressedResearchNotes *[]string, client *instructor.InstructorOpenAI, logger *slog.Logger) *ResearchReportGeneration {
	return &ResearchReportGeneration{
		client:                  client,
		logger:                  logger,
		researchBrief:           researchBrief,
		compressedResearchNotes: compressedResearchNotes,
	}
}

func (rrg *ResearchReportGeneration) Execute(ctx context.Context) (any, bool, error) {
	data := TemplateData{
		ResearchBrief:           *rrg.researchBrief,
		CompressedResearchNotes: *rrg.compressedResearchNotes,
	}
	prompt, err := PromptBuilder("research_report_generation", writeResearchReportPrompt, data)
	if err != nil {
		return "", false, fmt.Errorf("failed to build prompt: %w", err)
	}

	var ResearchReport ResearchReportGenerationOutputSchema
	resp, err := rrg.client.CreateChatCompletion(
		ctx, openai.ChatCompletionRequest{
			Model: openai.GPT5,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		}, &ResearchReport,
	)
	_ = resp
	if err != nil {
		return "", false, fmt.Errorf("failed to generate research report: %w", err)
	}
	return ResearchReport.Report, true, nil
}
