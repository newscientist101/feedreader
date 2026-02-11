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

// AIScraperGenerator generates scraper configs using Claude
type AIScraperGenerator struct {
	apiKey     string
	httpClient *http.Client
}

func NewAIScraperGenerator() *AIScraperGenerator {
	return &AIScraperGenerator{
		apiKey: os.Getenv("ANTHROPIC_API_KEY"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (g *AIScraperGenerator) IsAvailable() bool {
	return g.apiKey != ""
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

func (g *AIScraperGenerator) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if !g.IsAvailable() {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	// Fetch the page HTML
	htmlContent, err := g.fetchPage(ctx, req.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}

	// Truncate HTML if too long (keep first 50KB)
	if len(htmlContent) > 50000 {
		htmlContent = htmlContent[:50000] + "\n<!-- truncated -->"
	}

	// Build the prompt
	prompt := g.buildPrompt(req.URL, req.Description, htmlContent)

	// Call Claude API
	response, err := g.callClaude(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}

	return response, nil
}

func (g *AIScraperGenerator) fetchPage(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FeedReader/1.0)")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (g *AIScraperGenerator) buildPrompt(url, description, html string) string {
	return fmt.Sprintf(`You are a web scraping expert. Analyze this HTML page and create a scraper configuration to extract the requested data.

## Target URL
%s

## What to extract
%s

## HTML Content
%s

## Task
Create a JSON scraper configuration that extracts the requested items. The config uses regex patterns to match content.

The config format is:
{
  "itemPattern": "regex to match each item/article container",
  "titlePattern": "regex with capture group for title",
  "urlPattern": "regex with capture group for URL/link",
  "summaryPattern": "regex with capture group for summary/description (optional)",
  "authorPattern": "regex with capture group for author (optional)",
  "datePattern": "regex with capture group for date (optional)",
  "imagePattern": "regex with capture group for image URL (optional)",
  "baseUrl": "base URL for resolving relative links"
}

Important:
- Use (?s) flag for patterns that span multiple lines
- Use non-greedy quantifiers (.*?) to avoid over-matching
- Capture groups () extract the actual content
- Test patterns should work with Go's regexp package
- Only include patterns you're confident about

Respond with ONLY a JSON object in this exact format (no markdown, no explanation):
{
  "name": "suggested scraper name",
  "config": { ...the scraper config object... }
}`, url, description, html)
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (g *AIScraperGenerator) callClaude(ctx context.Context, prompt string) (*GenerateResponse, error) {
	reqBody := claudeRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 2000,
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if claudeResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", claudeResp.Error.Message)
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	// Parse the response JSON
	text := strings.TrimSpace(claudeResp.Content[0].Text)
	
	// Try to extract JSON if wrapped in markdown code blocks
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		text = strings.Join(jsonLines, "\n")
	}

	var result struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return &GenerateResponse{
			Error: fmt.Sprintf("Failed to parse AI response: %v\nRaw: %s", err, text),
		}, nil
	}

	// Pretty-print the config
	var configObj map[string]any
	if err := json.Unmarshal(result.Config, &configObj); err != nil {
		return &GenerateResponse{
			Name:   result.Name,
			Config: string(result.Config),
		}, nil
	}

	prettyConfig, _ := json.MarshalIndent(configObj, "", "  ")

	return &GenerateResponse{
		Name:   result.Name,
		Config: string(prettyConfig),
	}, nil
}
