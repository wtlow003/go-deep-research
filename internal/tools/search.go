package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sashabaranov/go-openai"
)

type SearchTool struct {
	Query string `json:"query" jsonschema:"title=search query,description=the search query to be use for web search,required"`
}

var SearchToolDefinition = openai.FunctionDefinition{
	Name:        "search_tool",
	Description: "Search the web for information",
	Parameters:  GenerateToolSchema[SearchTool](),
}

func (s SearchTool) Execute(input json.RawMessage) ([]string, error) {
	var searchInput SearchTool
	err := json.Unmarshal(input, &searchInput)
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse search input: %w", err)
	}

	requestPayload, err := json.Marshal(
		map[string]interface{}{
			"query":      searchInput.Query,
			"type":       "auto",
			"numResults": 10,
			"contents": map[string]interface{}{
				"text": true,
			},
		},
	)
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse search input: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequest("POST", "https://api.exa.ai/search", bytes.NewBuffer(requestPayload))
	if err != nil {
		return []string{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("x-api-key", os.Getenv("EXA_API_KEY"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
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

	var searchResult map[string]interface{}
	err = json.Unmarshal(body, &searchResult)
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var texts []string
	results := searchResult["results"].([]interface{})
	for _, result := range results {
		if text, ok := result.(map[string]interface{})["text"].(string); ok {
			texts = append(texts, text)
		}
	}
	return texts, nil
}
