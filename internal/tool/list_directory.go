package tool

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

// listDirectoryEntry returns the ToolEntry for the list_directory tool.
func listDirectoryEntry() ToolEntry {
	return ToolEntry{
		Definition: listDirectoryDefinition,
		Execute:    listDirectoryExecute,
	}
}

func listDirectoryDefinition() provider.Tool {
	return provider.Tool{
		Name:        toolname.ListDirectory,
		Description: "List the contents of a directory relative to the working directory. Returns one entry per line; subdirectories are suffixed with /.",
		Parameters: map[string]provider.ToolParameter{
			"path": {
				Type:        "string",
				Description: "Relative path to the directory to list. Use \".\" to list the working directory root.",
				Required:    true,
			},
			"artifact": {
				Type:        "string",
				Required:    false,
				Description: `When "true", list the artifact directory instead of the working directory.`,
			},
		},
	}
}

func listDirectoryExecute(ctx context.Context, call provider.ToolCall, ec ExecContext) (result provider.ToolResult) {
	path := call.Arguments["path"]

	defer func() {
		var summary string
		if result.IsError {
			summary = fmt.Sprintf("path %q: %s", path, result.Content)
		} else {
			summary = fmt.Sprintf("path %q", path)
		}
		toolVerboseLog(ec, toolname.ListDirectory, result, summary)
	}()

	// Determine which directory to resolve against.
	baseDir := ec.Workdir
	if strings.EqualFold(call.Arguments["artifact"], "true") {
		if ec.ArtifactDir == "" {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: "artifact directory not configured for this agent",
				IsError: true,
			}
		}
		baseDir = ec.ArtifactDir
	}
	resolved, err := validatePath(baseDir, path)
	if err != nil {
		msg := err.Error()
		if strings.EqualFold(call.Arguments["artifact"], "true") {
			msg = rewriteArtifactError(msg)
		}
		return provider.ToolResult{
			CallID:  call.ID,
			Content: msg,
			IsError: true,
		}
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: err.Error(),
			IsError: true,
		}
	}

	if len(entries) == 0 {
		return provider.ToolResult{
			CallID:  call.ID,
			Content: "",
			IsError: false,
		}
	}

	var b strings.Builder
	for i, entry := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(entry.Name())
		if entry.IsDir() {
			b.WriteByte('/')
		}
	}

	return provider.ToolResult{
		CallID:  call.ID,
		Content: b.String(),
		IsError: false,
	}
}
