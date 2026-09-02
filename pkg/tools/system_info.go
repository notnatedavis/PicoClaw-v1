//   pkg/tools/system_info.go

package tools

import (
	"context"
	"fmt"
	"runtime"
	"syscall"

	"github.com/picoclaw/pkg/llm"
)

// SystemInfoTool returns basic system information.
type SystemInfoTool struct{}

func (t *SystemInfoTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var disk syscall.Statfs_t
	err := syscall.Statfs(".", &disk)
	if err != nil {
		return nil, err
	}
	diskTotal := disk.Blocks * uint64(disk.Bsize)
	diskFree := disk.Bfree * uint64(disk.Bsize)

	return map[string]interface{}{
		"go_version":    runtime.Version(),
		"num_cpu":       runtime.NumCPU(),
		"memory_alloc_mb": mem.Alloc / 1024 / 1024,
		"memory_total_mb": mem.Sys / 1024 / 1024,
		"disk_total_gb": float64(diskTotal) / 1024 / 1024 / 1024,
		"disk_free_gb":  float64(diskFree) / 1024 / 1024 / 1024,
	}, nil
}

func (t *SystemInfoTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "system_info",
		Description: "Get system information: CPU count, memory usage, disk usage, and Go version.",
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}
}