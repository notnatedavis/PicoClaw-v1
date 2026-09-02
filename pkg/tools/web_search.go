//   pkg/tools/web_search.go

package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/picoclaw/pkg/llm"
)

// WebSearchTool searches the web using DuckDuckGo (no API key required).
type WebSearchTool struct{}

func (t *WebSearchTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("'query' parameter is required")
	}
	count := 5 // default
	if c, ok := params["count"].(float64); ok {
		count = int(c)
	}
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)
	// Extract result blocks: each result is inside <a class="result__a" href="...">Title</a>
	reLink := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	matches := reLink.FindAllStringSubmatch(html, -1)

	results := make([]map[string]string, 0, count)
	for _, m := range matches {
		if len(results) >= count {
			break
		}
		rawURL := m[1]
		title := stripHTML(m[2])
		// DuckDuckGo redirects URLs via uddg parameter, decode it
		if parsed, err := url.Parse(rawURL); err == nil {
			if uddg := parsed.Query().Get("uddg"); uddg != "" {
				rawURL = uddg
			}
		}
		results = append(results, map[string]string{
			"title": title,
			"url":   rawURL,
		})
	}
	if len(results) == 0 {
		return "No results found.", nil
	}
	return results, nil
}

func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}

func (t *WebSearchTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "web_search",
		Description: "Search the web using DuckDuckGo and return up to 10 results with titles and URLs.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
				"count": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 10},
			},
			"required": []string{"query"},
		},
	}
}