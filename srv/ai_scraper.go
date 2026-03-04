package srv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// aiScraperTimeout is the timeout for AI scraper generation requests.
// Used for the HTTP client, the poll loop, and the request context.
const aiScraperTimeout = 120 * time.Second

// ShelleyScraperGenerator generates scraper configs using the local Shelley API
type ShelleyScraperGenerator struct {
	shelleyURL string
	httpClient *http.Client
}

// NewShelleyScraperGenerator creates a Shelley API client. The URL can be
// overridden with the SHELLEY_URL environment variable.
func NewShelleyScraperGenerator() *ShelleyScraperGenerator {
	url := os.Getenv("SHELLEY_URL")
	if url == "" {
		url = "http://localhost:9999"
	}
	return &ShelleyScraperGenerator{
		shelleyURL: url,
		httpClient: &http.Client{
			Timeout: aiScraperTimeout,
		},
	}
}

func (g *ShelleyScraperGenerator) IsAvailable() bool {
	// Check if Shelley is running
	req, err := http.NewRequest("GET", g.shelleyURL+"/api/conversations", http.NoBody)
	if err != nil {
		return false
	}
	req.Header.Set("X-Exedev-Userid", "local")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == 200
}

type GenerateRequest struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type GenerateResponse struct {
	Name   string `json:"name"`
	Config string `json:"config"`
	Error  string `json:"error,omitempty"`
}

func (g *ShelleyScraperGenerator) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Create a new conversation with Shelley
	prompt := g.buildPrompt(req.URL, req.Description)

	// Start a new conversation
	convReq := map[string]string{
		"message": prompt,
		"cwd":     "/tmp",
	}
	jsonBody, _ := json.Marshal(convReq)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.shelleyURL+"/api/conversations/new", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Exedev-Userid", "local")
	httpReq.Header.Set("X-Shelley-Request", "true")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to contact Shelley: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("shelley returned error: %s", string(body))
	}

	var convResp struct {
		ConversationID string `json:"conversation_id"`
		Status         string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&convResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Poll for completion by checking the database directly
	result, err := g.waitForResponse(ctx, convResp.ConversationID)
	if err != nil {
		return nil, err
	}

	return g.parseResponse(result)
}

func (g *ShelleyScraperGenerator) buildPrompt(url, description string) string {
	return fmt.Sprintf(`I need you to create a scraper configuration for the FeedReader application.

Target URL: %s
What to extract: %s

Please:
1. Fetch the webpage at the URL above
2. Analyze the HTML structure
3. Create a JSON scraper configuration

The config format uses CSS selectors (parsed by goquery/cascadia):
{
  "type": "html",
  "itemSelector": "CSS selector for each item container",
  "titleSelector": "CSS selector for title (uses text content)",
  "urlSelector": "CSS selector for link element",
  "urlAttr": "attribute for URL (default: href)",
  "summarySelector": "CSS selector for summary text (optional)",
  "authorSelector": "CSS selector for author (optional)",
  "imageSelector": "CSS selector for image (optional)",
  "imageAttr": "attribute for image URL (default: src)",
  "dateSelector": "CSS selector for date element (optional)",
  "dateAttr": "attribute for date value (optional, uses text if empty)",
  "baseUrl": "base URL for resolving relative links"
}

All selectors are relative to the matched itemSelector element.
Example: if itemSelector is "div.post" and titleSelector is "h2 a", it finds h2 a inside each div.post.

Respond with ONLY a JSON object in this exact format (no explanation, no markdown code blocks):
{"name": "suggested scraper name", "config": { ...the config... }}`, url, description)
}

func (g *ShelleyScraperGenerator) waitForResponse(ctx context.Context, conversationID string) (string, error) {
	// Poll for the agent response using the API
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(aiScraperTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for Shelley response")
		case <-ticker.C:
			// Check if conversation is done via API
			req, _ := http.NewRequestWithContext(ctx, "GET", g.shelleyURL+"/api/conversations", http.NoBody)
			req.Header.Set("X-Exedev-Userid", "local")
			resp, err := g.httpClient.Do(req)
			if err != nil {
				continue
			}

			var convs []struct {
				ConversationID string `json:"conversation_id"`
				Working        bool   `json:"working"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&convs); err != nil {
				_ = resp.Body.Close()
				continue
			}
			_ = resp.Body.Close()

			working := true
			for _, c := range convs {
				if c.ConversationID == conversationID {
					working = c.Working
					break
				}
			}

			if !working {
				// Get the response via the API
				return g.getResponseFromAPI(ctx, conversationID)
			}
		}
	}
}

func (g *ShelleyScraperGenerator) getResponseFromAPI(ctx context.Context, conversationID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", g.shelleyURL+"/api/conversation/"+conversationID, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Exedev-Userid", "local")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch conversation: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("shelley API returned status %d", resp.StatusCode)
	}

	var convData struct {
		Messages []struct {
			Type    string `json:"type"`
			LLMData string `json:"llm_data"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&convData); err != nil {
		return "", fmt.Errorf("failed to decode conversation: %w", err)
	}

	var allText strings.Builder
	for _, msg := range convData.Messages {
		if msg.Type != "agent" || msg.LLMData == "" {
			continue
		}

		// Parse the LLM data to extract text
		var llm struct {
			Content []struct {
				Type int    `json:"Type"`
				Text string `json:"Text"`
			} `json:"Content"`
		}
		if err := json.Unmarshal([]byte(msg.LLMData), &llm); err != nil {
			continue
		}

		for _, content := range llm.Content {
			// Type 2 is text content
			if content.Type == 2 && content.Text != "" {
				allText.WriteString(content.Text)
				allText.WriteString("\n")
			}
		}
	}

	result := allText.String()
	if result == "" {
		return "", fmt.Errorf("empty response from Shelley")
	}
	return result, nil
}

func (g *ShelleyScraperGenerator) parseResponse(text string) (*GenerateResponse, error) {
	text = strings.TrimSpace(text)

	// Try to find JSON in the response
	// Look for the pattern {"name":...}
	start := strings.Index(text, `{"name"`)
	if start == -1 {
		// Try alternate format
		start = strings.Index(text, `{ "name"`)
	}
	if start == -1 {
		return &GenerateResponse{
			Error: fmt.Sprintf("Could not find JSON in response. Raw response:\n%s", truncate(text, 500)),
		}, nil
	}

	// Find matching closing brace
	text = text[start:]
	depth := 0
	end := -1
	for i, ch := range text {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if end == -1 {
		return &GenerateResponse{
			Error: "Could not parse JSON from response",
		}, nil
	}

	jsonStr := text[:end]

	var result struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return &GenerateResponse{
			Error: fmt.Sprintf("Failed to parse JSON: %v", err),
		}, nil
	}

	// Pretty-print the config
	var configObj map[string]any
	if err := json.Unmarshal(result.Config, &configObj); err != nil {
		return &GenerateResponse{
			Name:   result.Name,
			Config: string(result.Config),
		}, err
	}

	prettyConfig, _ := json.MarshalIndent(configObj, "", "  ")

	return &GenerateResponse{
		Name:   result.Name,
		Config: string(prettyConfig),
	}, nil
}
