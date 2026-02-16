package srv

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ShelleyScraperGenerator generates scraper configs using the local Shelley API
type ShelleyScraperGenerator struct {
	shelleyURL string
	httpClient *http.Client
	dbPath     string
}

func NewShelleyScraperGenerator() *ShelleyScraperGenerator {
	return &ShelleyScraperGenerator{
		shelleyURL: "http://localhost:9999",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		dbPath: os.ExpandEnv("$HOME/.config/shelley/shelley.db"),
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()

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

	timeout := time.After(120 * time.Second)

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
				resp.Body.Close()
				continue
			}
			resp.Body.Close()

			working := true
			for _, c := range convs {
				if c.ConversationID == conversationID {
					working = c.Working
					break
				}
			}

			if !working {
				// Get the response from the database
				return g.getResponseFromDB(ctx, conversationID)
			}
		}
	}
}

func (g *ShelleyScraperGenerator) getResponseFromDB(ctx context.Context, conversationID string) (string, error) {
	db, err := sql.Open("sqlite", g.dbPath+"?mode=ro")
	if err != nil {
		return "", fmt.Errorf("failed to open Shelley DB: %w", err)
	}
	defer db.Close()

	// Get all agent messages and concatenate text
	rows, err := db.QueryContext(ctx,
		`SELECT llm_data FROM messages 
		 WHERE conversation_id = ? AND type = 'agent' 
		 ORDER BY sequence_id`,
		conversationID)
	if err != nil {
		return "", fmt.Errorf("failed to get response: %w", err)
	}
	defer rows.Close()

	var allText strings.Builder
	for rows.Next() {
		var llmData string
		if err := rows.Scan(&llmData); err != nil {
			continue
		}

		// Parse the LLM data to extract text
		var msg struct {
			Content []struct {
				Type int    `json:"Type"`
				Text string `json:"Text"`
			} `json:"Content"`
		}
		if err := json.Unmarshal([]byte(llmData), &msg); err != nil {
			continue
		}

		for _, content := range msg.Content {
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
