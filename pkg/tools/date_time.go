//   pkg/tools/date_time.go

package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/picoclaw/pkg/llm"
)

// DateTimeTool provides current local date and time.
type DateTimeTool struct{}

func (t *DateTimeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	format, _ := params["format"].(string)
	now := time.Now()
	switch format {
	case "unix":
		return fmt.Sprintf("%d", now.Unix()), nil
	case "date":
		return now.Format("2006-01-02"), nil
	case "time":
		return now.Format("15:04:05"), nil
	default:
		return now.Format(time.RFC3339), nil
	}
}

func (t *DateTimeTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "date_time",
		Description: "Get current local date and time. Optional 'format' can be 'unix', 'date', 'time', or empty for full RFC3339.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type": "string",
					"enum": []string{"unix", "date", "time", "rfc3339"},
				},
			},
		},
	}
}