//   pkg/tools/file_search.go

package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/picoclaw/pkg/llm"
)

// FileSearchTool searches for a pattern inside files within a base directory.
type FileSearchTool struct {
	baseDir string
}

func NewFileSearchTool(baseDir string) *FileSearchTool {
	return &FileSearchTool{baseDir: baseDir}
}

func (t *FileSearchTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("'query' is required")
	}
	maxResults := 10
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	}
	if maxResults < 1 {
		maxResults = 1
	}
	if maxResults > 50 {
		maxResults = 50
	}

	results := []map[string]interface{}{}
	err := filepath.Walk(t.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore errors
		}
		if info.IsDir() {
			return nil
		}
		// Skip binary files (simple check)
		if isBinaryFile(path) {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if strings.Contains(scanner.Text(), query) {
				relPath, _ := filepath.Rel(t.baseDir, path)
				results = append(results, map[string]interface{}{
					"file":    relPath,
					"line":    lineNum,
					"content": strings.TrimSpace(scanner.Text()),
				})
				if len(results) >= maxResults {
					return filepath.SkipAll // stop walking
				}
			}
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return nil, err
	}
	if len(results) == 0 {
		return "No matches found.", nil
	}
	return results, nil
}

func isBinaryFile(path string) bool {
	// Simple heuristic: check first 512 bytes for null bytes
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return bytes.IndexByte(buf[:n], 0) != -1
}

func (t *FileSearchTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "file_search",
		Description: "Search for a substring in files under the agent's workspace. Returns matching file paths, line numbers, and snippet.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":       map[string]interface{}{"type": "string"},
				"max_results": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 50},
			},
			"required": []string{"query"},
		},
	}
}