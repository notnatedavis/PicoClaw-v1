//   pkg/tools/http_request.go

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/picoclaw/pkg/llm"
)

// HTTPRequestTool makes arbitrary HTTP requests (GET/POST).
type HTTPRequestTool struct{}

func (t *HTTPRequestTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	urlStr, _ := params["url"].(string)
	if urlStr == "" {
		return nil, fmt.Errorf("'url' is required")
	}

	var body io.Reader
	if b, ok := params["body"].(string); ok && b != "" {
		body = bytes.NewBufferString(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, err
	}

	// Optional headers as map[string]string
	if headers, ok := params["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try to parse JSON, otherwise return raw string
	var jsonBody interface{}
	if err := json.Unmarshal(respBody, &jsonBody); err == nil {
		return map[string]interface{}{
			"status_code": resp.StatusCode,
			"headers":     resp.Header,
			"body":        jsonBody,
		}, nil
	}

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
	}, nil
}

func (t *HTTPRequestTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "http_request",
		Description: "Perform an HTTP request (GET or POST) and return status code, headers, and response body.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"method": map[string]interface{}{
					"type": "string",
					"enum": []string{"GET", "POST"},
				},
				"url":     map[string]interface{}{"type": "string"},
				"body":    map[string]interface{}{"type": "string"},
				"headers": map[string]interface{}{"type": "object"},
			},
			"required": []string{"url"},
		},
	}
}