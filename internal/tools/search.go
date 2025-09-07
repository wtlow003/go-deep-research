package tools

import (
	"bytes"
	"deep-research/internal/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
)

type SearchTool struct {
	Query string `json:"query" jsonschema:"title=search query,description=the search query to be use for web search,required"`
}

type exaClient struct {
	client *http.Client
}

type exaSearchRequest struct {
	Query      string         `json:"query"`
	Type       string         `json:"type"`
	NumResults int            `json:"numResults"`
	Content    map[string]any `json:"contents"`
}

type exaSearchResponse struct {
	Results []exaIndividualSearchResult `json:"results"`
}

type exaIndividualSearchResult struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Url           string `json:"url"`
	PublishedDate string `json:"publishedDate"`
	Author        string `json:"author"`
	Text          string `json:"text"`
	Image         string `json:"image"`
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

var defaultExaClient *exaClient

var SearchToolDefinition = openai.FunctionDefinition{
	Name:        "search_tool",
	Description: "Search the web for information",
	Parameters:  GenerateToolSchema[SearchTool](),
}

func getExaClient() *exaClient {
	if defaultExaClient == nil {
		defaultExaClient = &exaClient{
			client: httpClient,
		}
	}
	return defaultExaClient
}

func (e *exaClient) Search(cfg *config.Config, query []byte) ([]string, error) {
	req, err := http.NewRequest("POST", cfg.ExaEndpoint, bytes.NewBuffer(query))
	if err != nil {
		return []string{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("x-api-key", cfg.ExaKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return []string{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			fmt.Printf("failed to close response body: %v\n", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var searchResult exaSearchResponse
	err = json.Unmarshal(body, &searchResult)
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var texts []string
	results := searchResult.Results
	for _, result := range results {
		texts = append(texts, result.Text)
	}
	return texts, nil
}

func (s SearchTool) Execute(input json.RawMessage) ([]string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return []string{}, fmt.Errorf("failed to load configuration: %w", err)
	}

	var searchInput SearchTool
	err = json.Unmarshal(input, &searchInput)
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse search input: %w", err)
	}

	requestPayload, err := json.Marshal(
		&exaSearchRequest{
			Query:      searchInput.Query,
			Type:       "auto",
			NumResults: cfg.ExaNumSearchResult,
			Content: map[string]any{
				"text": true,
			},
		},
	)

	if err != nil {
		return []string{}, fmt.Errorf("failed to parse search input: %w", err)
	}

	results, err := getExaClient().Search(cfg, requestPayload)
	if err != nil {
		return []string{}, fmt.Errorf("failed to search: %w", err)
	}
	return results, nil
}
